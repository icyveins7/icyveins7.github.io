---
title: "Some tips for integrating small bits of CUDA code into larger codebases"
date: 2025-08-13T20:00:00+08:00
draft: false
toc: false
images:
tags:
  - "cuda"
  - "c++"
  - "pimpl"
---

> Some lessons from general C++ come in handy here..

In some recent work I integrated some CUDA code I developed into a larger existing codebase, written for the CPU. Essentially, my module(s) would accelerate and replace some existing functionality, but was only a small cog in the machine.

This is likely applicable to many others, so hopefully the lessons I document here will be concisely useful to those who chance upon this post.

# The obvious bits

During my kernel development, I pulled out the necessary functionality and experimented on it in a small standalone project, like any sane person. Like most of my personal projects, this lets me use CMake, but the larger codebase was built on (mostly handmade) raw Makefiles.

Inspecting `compile_commands.json` produced by CMake will reveal two common compiler options used for all CUDA code:

1. `-forward-unknown-to-host-compiler`, [which does what it says on the tin](https://docs.nvidia.com/cuda/cuda-compiler-driver-nvcc/#forward-unknown-to-host-compiler-forward-unknown-to-host-compiler). You can also use the [lazier form of this](https://docs.nvidia.com/cuda/cuda-compiler-driver-nvcc/#forward-unknown-opts-forward-unknown-opts), `-forward-unknown-opts`, though I don't usually bother until linker errors start showing up.
2. `-x cu` for every source file. This isn't technically required for `.cu` files since `nvcc` will recognise them as CUDA files, but becomes important when you place your eventual code into existing `.cpp` files. [This ensures they are compiled as CUDA](https://docs.nvidia.com/cuda/cuda-compiler-driver-nvcc/#x-c-c-cu-x).
# Relocatable device code requirements

The codebase is also partitioned into many separate distinct libraries, compiled individually and then linked together at the end to create one final executable. Of course, my standalone project with its multiple small individual executables and unit tests didn't need any extra compiler flags, but integrating the code as it was made `nvcc` cry at me.

This may have been partly due to the structure in the codebase itself, but the solution was fairly simple:

> Add `-dc` to the `nvcc`'s compiler flags, turning on [relocatable device code with the expectation of linkage later](https://docs.nvidia.com/cuda/cuda-compiler-driver-nvcc/#device-c-dc).

We will be returning to the docs for `nvcc` often; get used to it, because that's how I (you) will figure out what flags you need to add.

# Macro pollution

This compiler error is strictly due to existing code in the existing codebase, so you may not experience anything similar. In fact, it is not strictly a CUDA related issue, but could occur with any code.

The observed error was something like this, upon inclusion of anything that used `cub`, either directly or indirectly, like via `thrust` headers:

```
C:\Program Files\NVIDIA GPU Computing Toolkit\CUDA\v12.9\bin/../include\cub/warp/specializations/warp_scan_shfl.cuh(408): error: identifier "input" is undefined
    __declspec(__device__) __forceinline  InclusiveScanStep( input, ScanOpT scan_op, int first_lane, int offset)    
                                                             ^
```

Note that this never occurred in my standalone project, so it was odd that these headers would suddenly cause problems here.

As mentioned above, this isn't specifically a CUDA issue, but I'll show how I investigated it in the context of using `nvcc`.

## Read the preprocessor output

We can [dump all intermediate output from `nvcc`](https://docs.nvidia.com/cuda/cuda-compiler-driver-nvcc/#keep-keep) via `-keep`. This lets you see, among many other things, the preprocessor output in a file usually suffixed with `.ii`.

Opening it in my case showed that the input argument's template argument, `_T`, had somehow been stripped away.

```cpp
// note: not the exact same function as the above one, but obvious that the first template argument is gone
template <typename , typename ScanOpT>
__declspec(__device__) __forceinline 
InclusiveScanStep( input, ScanOpT scan_op, int first_lane, int offset, ::cuda::std::true_type )
{
  return InclusiveScanStep(input, scan_op, first_lane, offset);
}
```

This smells of macro replacement, but where is it coming from? Searching the original preprocessor output file for `#define _T` didn't seem to yield anything.

I had to enable the `gcc`-specific option `-dD` in order to [enable the `define` directives](https://gcc.gnu.org/onlinedocs/gcc/Preprocessor-Options.html#index-dD). Then I was able to track down an include file several levels up which happened to do exactly `#define _T`. Welp, there goes all the `cub` headers I guess.

A simple `#undef _T` in my translation unit and I was back on track.
# Hiding CUDA from the rest of the codebase

Congrats for making it this far down the post. Below lies the meat of the problems I had. Almost all my issues stemmed from having to do this, simply because it would be too gargantuan a task to refactor everything else.

I've already mentioned above that compiling `.cpp` files as CUDA requires the `nvcc` option `-x cu`; in fact, this is what CMake does overtly, even for `.cu` files, once you use

```
set_source_files_properties(sourcefile.cu PROPERTIES LANGUAGE CUDA)
```

as seen in many CUDA examples.
## Why you should specify source file compilation explicitly

Aside from the fact that your code probably won't compile, let's go into exactly why that actually happens, and why using your host compiler doesn't work:

1. *Device code will not be parsable.* This should be obvious, but it is important to note that this will also happen if you have a header-only template calling a kernel, having some `__device__` function, or any other CUDA-specific syntax. This will become important later.
2. *CUDA headers and libraries will not get automatically found*. Now this is arguably fixable when the project is small by explicitly adding CUDA header and library paths to the Makefiles (or equivalent), but this quickly gets unwieldy once the project starts getting larger. There is also no guarantee that all the CUDA headers will just work; some may include some `__device__` code, for example.
3. *Hyper-aggressive inlining of code.* This is, of course, preferable in most instances of device code, but can cause issues in some CPU-side code sometimes. Fairly rare, but this was observed for one file in my case.

The above probably isn't a definitive list, but hopefully it's convincing enough. They are all underscored by a general software idea though: **separation of concerns**. We shouldn't use `nvcc` for things unless strictly necessary.

## But including CUDA headers implies parsing them

Here's the problem. You've already developed some CUDA code: some kernels, and some encapsulated classes to help call them and/or manage some device-side states and workspaces. Perhaps you also want some of the external code to hold on to some memory through `thrust::host_vector` or `thrust::device_vector`.

Normally, it wouldn't matter much (unless you are extremely pedantic) where you included headers like `thrust/device_vector.h`. However, with our `nvcc`-specific constraints, this becomes a bit more nuanced.

> *You can no longer make these includes in the headers of your external code, or else all other translation units which transitively include the header will need to be compiled by `nvcc`*.

This is probably not a good idea, even if you think you are okay with the maintenance.

## A transitive inclusion example

I can give a MWO of my scenario. Consider the following implementation for a class in the larger codebase:

```cpp
// codebaseclass.h
#include <thrust/device_vector.h>

class CodebaseClass
{
// ...

private:
  thrust::device_vector<int> m_vec;
}

// codebaseclass.cpp
#include "codebaseclass.h"
// all the implementation details for CodebaseClass in here
// ...
```

Now we set our build system to `nvcc` for `codebaseclass.cpp`, and we should be good to go, right?

No, but you forgot that somewhere else in the codebase, something else uses this header:

```cpp
// othercodebase.h

#include "codebaseclass.h"

// ...

// othercodebase.cpp
#include "othercodebase.h"

// ...
```

Now when the compiler sees the translation unit for `othercodebase.cpp`, it pulls in `codebaseclass.h` as well, and doesn't know how to deal with the `thrust` vector.

# pImpl-ing our way through

The pImpl pattern isn't new, nor is it technically meant for purposes like this; it is meant to hide functionality from a client interface. Here, we're using it to hide the functionality from our compilers.

The idea is to simply forward declare our CUDA-classes (including any that we have defined that use CUDA things), and only store pointers to our objects, rather than the objects themselves.

```cpp
// codebaseclass.h

// forward declaration
namespace thrust{
  template <typename T>
  class device_vector<T>;
}

class CodebaseClass{
public:
  CodebaseClass();
  ~CodebaseClass();
private:
  std::unique_ptr<thrust::device_vector> m_vec;
}

// codebaseclass.cpp
#include "codebaseclass.h"

#include <thrust/device_vector.h>

// ... use the device vector as per normal, just that it's via a pointer

```

Now, transitively including `codebaseclass.h` into `othercodebase.h` does not cause problems, because a pointer is simply an address to the compiler. It expects that if you *actually need to use it*, you will link it in yourself later.

> Yes, technically pImpl is a lot more than this, and usually involves a class-internal forward declaration which gets inherited from. We don't really need all that here, and pImpl-ing has a nicer ring to it than 'forward-declaring'.

## Why this works, and when it doesn't

This goes back to compiler fundamentals, and essentially boils down to what information the compiler needs when a full type is specified.

When the entire class object is a direct member of another class, like I wrote originally, the compiler must know how to construct and destroy it. This is because its inclusion as a member variable implies that it must be default constructed. Without the class header/implementation like `thrust/device_vector.h` in this case, it will be unable to do this.

Similarly, you wouldn't be able to call any methods directly without first having its definition e.g. having an inline function that calls `m_vec->reserve()`. In my above example, we just store the pointer to it, which is basically saying "*store an address first, I'll tell you what it does later, don't worry about it*".

Now there are a few additional things to note when we implement this pattern. Notice how I *did not define the destructor inline within the header*. This isn't an arbitrary decision; the code will not compile if we had done so i.e. written the innocuous-looking

```cpp
~CodebaseClass(){}
```

**even if we were going to do this anyway in the source file!**

Why does this happen? Because the compiler will emit full code to destroy all objects at the point where the destructor is *defined*. So if you define it inline in the header, it *must know how to destroy `thrust::device_vector`, and since we only forward declared it, it will not know how to*.

The two options in this case are to:
1. Define the destructor out-of-line i.e. in the associated `.cpp`, by which point the compiler should have full knowledge of the `thrust::device_vector` type. This is *particularly important for polymorphic classes* since the compiler does not auto-generate a virtual destructor, so you must always declare it in the header at least, but *not* define it there.
2. Simply not defining it at all. The compiler will emit it after it has the full definition of the encompassing class, which occurs in the `.cpp`, same as the above.

> Note that the second version is *not* equivalent to writing `~CodebaseClass() = default` in the header, as the compiler will then attempt to emit it in the header again, and the same problem will arise. You can, however, declare it in the header *and then write `~CodebaseClass() = default;`* in the source `.cpp` file.

## If you really want the full pImpl experience..

Then you can use constructor inheritance to get you most of the way there. In these scenarios you would actually forward declare a `Impl` class or struct for whatever it is you want to hide.

A lot of the time, you don't really need to care about the constructor for this hidden implementation, and you just want to use your hidden class's constructor directly. Then something like this in the `.cpp` would work:

```cpp
struct Impl : public RealClass{
  using RealClass::RealClass; // this inherits all the constructors
  // ...
}
```

Again, this isn't a strictly CUDA thing, but it's useful to remember it in this context.
