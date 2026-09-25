---
title: "Cross-platform trigonometric SIMD and how the C ABI confused me"
date: 2025-01-30T20:00:00+08:00
draft: false
toc: false
images:
tags:
  - "c++"
  - "c"
  - "simd"
  - "svml"
  - "intrinsic"
---

> My small journey in discovering libmvec functions and the C problems I encountered..

I was recently working on making an existing Windows-only library of SIMD functions cross-platform i.e. making everything work in Linux. Here I'm going to highlight some problems I encountered in the process; hopefully it will help someone if they encounter something similar too.

# The Windows (MSVC) function

Intrinsics are usually tied to compiler implementations. The one that gave me issues was a bunch of trigonometric functions. Let's use the SSE version that works on floats; in MSVC this was implemented as a simple

```cpp
__m128 _mm_cos_ps(__m128 a)
```

This [function](https://www.intel.com/content/www/us/en/docs/intrinsics-guide/index.html#text=mm_cos_ps&ig_expand=1388), and others like it, simply do not exist in GCC implementations, at least not explicitly. As such, there is no way to direct a call an SIMD vector to a _simple_ `cos()` function for evaluation. Why I highlighted _simple_ will become clear in a minute.

# How GCC handles it

GCC does, of course, have a way to vectorise these operations through its `-ffast-math` flag. Using this and a standard `-O3`, we can see that a simple function like

```cpp
void manyCos(const float *x, float* y)
{
	for (int i = 0; i < 4000; ++i)
		y[i] = cosf(x[i]);
}
```

gets turned into assembly that looks like

```asm
lea rdx, [rdi+4]
mov rax, rsi
push r12
mov r12, rsi
sub rax, rdx
push rbp
mov rbp, rdi
push rbx
xor ebx, ebx
cmp rax, 8
jbe .L2

.L3:
movups xmm0, XMMWORD PTR [rbp+0+rbx]
call _ZGVbN4v_cosf
movups XMMWORD PTR [r12+rbx], xmm0
add rbx, 16
cmp rbx, 16000
jne .L3

.L1:
pop rbx
pop rbp
pop r12
ret

.L2:
movss xmm0, DWORD PTR [rbp+0+rbx]
call cosf
movss DWORD PTR [r12+rbx], xmm0
add rbx, 4
cmp rbx, 16000
jne .L2
jmp .L1
```

Now we can see this calls a particular intrinsic that looks like

```asm
_ZGVbN4v_cosf
```

This is a special routine that comes from `libmvec`, which is [a vector library that comes coupled inside glibc](https://sourceware.org/glibc/wiki/libmvec). GCC automatically uses this library when it is asked to vectorise more involved mathematical functions like `cosf`, `sinf`, `logf` etc (and their double variants of course).

# But how do we _manually_ call `libmvec` functions then?

The link above should make it clear that there is no intended use-case to manually call stuff in `libmvec`. Indeed, the vector ABI it uses also doesn’t correspond to the typical C++ name-mangling of symbol names.

Googling it doesn’t really come up with much information either. I have found that the best way in my opinion is to simply disassemble code compiled with `-ffast-math`, like above, and look for the function called. I can easily do this in [godbolt.org](godbolt.org) so that’s usually where I do it (and it doesn’t seem like the function name has/will change in the foreseeable future).

# Okay, we have the function name, so how do we call it?

The first thing to do is to include the header. This is simply in `math.h`, which is where you would find the normal math functions like `cosf()` anyway.

Now the next thing that is required is to make it `extern "C"`, because the library is written as a C library. __If you don't do this and you're compiling with `g++`, then C++ mangling is going to mangle the name again and it will result in undefined references.__

Alright, but what about the input argument types? It's meant to be used on `gcc`'s in-built vector type definitions like `__m128`, so that's what we will use. But here is where my problem began.

# Type qualifiers? Reference?

In a typical C++ function nowadays, it is usually recommended that input functions that are not modified in the function be declared as `const`, and that where size may matter, references (or pointers) be used. As such, my declaration looked like this:

```cpp
extern "C" __m128 _ZGVbN4v_cosf(const __m128&);
```

As it turns out, using a reference via `&` is completely incorrect, and very dangerous; this will compile without error, but will then produce nonsensical output at runtime. _The `const` is technically harmless, but is also irrelevant to this discussion anyway (since we are not the ones writing the function implementation)_.

The correct declaration is simply

```cpp
extern "C" __m128 _ZGVbN4v_cosf(__m128);
```

where the `const` is irrelevant.

# The C ABI (and the compiler) cannot know your mistake

I'll leave a [link here to my question in StackOverflow, with the fantastic answer by the assembly master Peter Cordes](https://stackoverflow.com/questions/79364070/output-errors-when-using-libmvec-intrinsics-for-trigo-functions-manually-like-c). I'll try to summarise his answer here.

Effectively, telling the compiler that a reference is input is the same as telling it to load a pointer address into the input registers of the function. Doing so, and then _wrongly_ calling the function with the C++-like syntax for references (where you just write the variable name), would result in the compiler taking the variable's value as the address.

I also asked this question to the experts in the Compiler Explorer discord, and @dragonmux was kind enough to respond. The explanation given to me there helped to clarify some things.

I first asked why the compiler couldn't help to catch it as I was still in my C++ mindset. For example, having two files with

```cpp
// # a.cpp
extern float func(float x){
	return x*2.0f + 0.1f;
}

// # b.cpp
// Forward declaration with 'wrong' input qualifier (added &)
extern float func(float& x);

// create a variable and explicit ref
float x = 2.5f;
float& xref = x;

// pass explicit ref to func
func(xref);
```

will fail to compile as `g++` will correctly identify that the reference is undefined (because it has retained the type qualifier information in the name-mangling).

If I instead now qualify everything with `extern "C"` instead,

```cpp
// # a.cpp
extern "C" float func(float x){
	return x*2.0f + 0.1f;
}

// # b.cpp
// Forward declaration with 'wrong' input qualifier (added &)
extern "C" float func(float& x);

// create a variable and explicit ref
float x = 2.5f;
float& xref = x;

// pass explicit ref to func
func(xref);
```

this now _fully compiles_, since the C ABI discards the input types, so the symbols are identical in both translation units. But then it __WILL__ crash (or in my contrived experiment it just outputs the wrong value `0.1f`), since we are effectively passing a pointer to something that expects a value.

Somehow, even though I was aware of name-mangling in C++, I didn't stop to think how it was protecting me from things like this.

Essentially, in the C++, non-`extern "C"` case, we have

1. A symbol for `func` in `a.cpp`, name-mangled to indicate the pure value argument.
2. A symbol for `func` in `b.cpp`, name-mangled to indicate the reference argument.
3. Linker attempts to find definition for symbol used in (2) from (1) and fails.

In the C version, everything links, because both symbols just look like `func`. Then at runtime, undefined behaviour results.

# Final question: is it bad/slow that the library is using SIMD register values instead of references?

In my same StackOverflow question above, Peter Cordes explained the nuances behind this pretty well, so go back to his answer directly if you want.

TL; DR, it's important to note that it's 

> cheap to pass/return by value in a single register.

Think of it just like any other POD like an `int`.

There are other details on how the vector registers are handled in cases like this, where the SVML (which includes things like the trigonometric functions for SIMD) aren't really hardware intrinsics, so they have to respect the

> x86-64 System V calling convention: there are no call-preserved vector registers.

and that means that

> only the return-value register can be relied on to have a useful value

but I think that's not really necessary to understand deeply from a library maintainer's perspective. You probably just want to know that the way you're calling the equivalent `cosf` intrinsic is the best way possible. __The answer is yes, doing it by value is indeed the best way possible.__
