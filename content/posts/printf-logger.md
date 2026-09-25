---
title: "Who knew a simple logger class would be this complicated?"
date: 2024-09-09T20:00:00+08:00
draft: false
toc: false
images:
tags:
  - "c++"
  - "c++20"
  - "printf"
  - "logger"
  - "source_location"
---

> Writing a printf-based C++ logger class was more of a journey than I originally thought..

I recently had to work with a codebase where the build process was so convoluted it couldn’t be run and debugged from within a Visual Studio instance, *despite being a Visual Studio solution*.

This was primarily because it had a bunch of Java components, which were mainly used for the UI, and that prevented it from being run as a standard C++ application within the debugger (or maybe you could? I couldn’t find a way..)

Regardless, the *normal* way - the way that everyone else working on it would do it - to debug was through logging. I had to make do, so I hopped on the faulty bandwagon.

# `printf` enjoyers unite

Alright alright, it’s not C++, but I honestly think C-like printf with its format specifier syntax is the cleanest, most concise way to log different POD-type values.

```c
printf(“a: %.6f\nb: %d\nc: %.2g\n”, a, b, c);
```

The equivalent of the above in C++ with stream-like things:

```cpp
std::cout << “a: “ << std::fixed << std::setprecision(6) << a << std::endl << “b: “ << b << std::endl << “c: “ << std::scientific << std::setprecision(2) << c << std::endl;
```

Holy shit, what a monster. Even if you remove every namespace call to `std::` and replace the `endl` calls with the newline characters, you’d still be sitting with a disgustingly long line of code.

It gets slightly better with C++20 and `std::fmt`:

```cpp
std::cout << std::format(“a: {:%.6f}\nb: {:%d}\nc: {:%.2g}\n”, a, b, c);
```

But still, why write an extra function call to `std::format` and extra braces when I didn’t have to previously? No one is going to change my mind on this. *Yes, I know it provides a lot more functionality, but I don’t really care for all those bells and whistles.*

# `printf`, but with RAII

Alright let’s just assume we want to have `printf` style logs, but still have access to the C-like defines in `__FILE__` and `__LINE__`. That let us print things like

```
/some/path/badfile.cpp:237
```

by simply using some macro magic like 

```cpp
#define log(…) my_logger_func(__FILE__, __LINE__, __VA_ARGS__)

// on line 237 of badfile.cpp
log(“some msg”);
```

Libraries like [this amazing one](https://github.com/rxi/log.c) do exactly that.

But I wanted to encapsulate some other things, like the automatic closing of a log file upon destruction.

Here’s the first problem if you make a class and provide a method to log, instead of a macro; the preprocessor magic no longer works, because the substitution for `__FILE__` and `__LINE__` will now occur directly where it was written (inside your class method) instead of where you *called the method*. 

```cpp
// logger.h
struct Logger
{
…
// At line 25 for example
void log(…){
  printf(“%s %d: some log message.\n”, __FILE__, __LINE__);
}
…
}

// main.cpp
…
logger.log(…);
…
// This will always print 
// logger.h 25: some log message, regardless of where it’s called
```

This is obviously not what we want. The only way around this would have been to provide `__FILE__` and `__LINE__` on every log method call, which would obviously be a far worse experience.

```cpp
// no one is going to do this
lgr.log(__FILE__, __LINE__, “what i actually want to log”);
```

# C++20 and `std::source_location`

I said *‘would have been’* because in C++20 we now have a useful alternative with `std::source_location`. Now we can do this:

```cpp
struct Logger{
  void log(std::source_location l = std::source_location::current()){
    printf(“%s:%d”, l.file_name(), l.line());
  }
}
```

The default argument gets substituted at the point the method is called, and we get back our true filename and line numbers!

But there’s now another problem: dealing with multiple, variable arguments for the substitutions.

In the world of C, we had `__VA_ARGS__`, or the corresponding variadic function helpers (see [this](https://en.cppreference.com/w/c/variadic)). In C++, we have similar functionality with variadic templates (see [this](https://en.cppreference.com/w/cpp/language/parameter_pack)), also known as parameter packs.

Now this is great - we can do a variable number of arbitrarily typed substitutions, `printf` style - but there’s a catch.

Both of these requirements need to be at the *end* of the function signature. We need the default argument to be at the end, because otherwise it won’t compile. 

```cpp
struct Logger
{
    // this doesn’t compile
    template <typename… Args>
    void log(std::source_location l = std::source_location::current(), Args… args)
    {
      …
    }
}
```

But we also need the parameter pack to be at the end, otherwise it won’t be able to automatically predict the template types for us:

```cpp
struct Logger
{
    // this may compile, but may give unexpected results
    // because the final argument in the function call 
    // will be directed to the std::source_location variable
    // in most cases it will fail since the supplied 
    // argument cannot be converted to a std::source_location
    template <typename… Args>
    void log(const char* s, Args… args, std::source_location l = std::source_location::current())
    {
      …
    }
}

// main.cpp
lgr.log(“my custom message %d %d”, lognumber, mylogvariable);
// the compiler will (attempt to) assign mylogvariable
// to replace the default std::source_location argument
```

There’s a very good discussion of this exact problem - which I referenced while coming up with my own flavour of a solution - [here](https://www.cppstories.com/2021/non-terminal-variadic-args/). For example, one way to ensure that the parameter unpacking template works as intended is to specify the exact types:

```cpp
lgr.log<int, int>(“my custom message %d %d”, lognumber, mylogvariable);
// this will print what you would expect
```

But this is a lot more verbose than I would like; it should be clear that with 10 variable substitutions you’d need to specify 10 types, so it can quickly get out of hand.

# The most concise compromise I could tolerate

My number one goal was to ensure as few characters typed, when compared to the original `printf` form.

I decided to use the ‘constructor template’ to achieve this. The final code looks something like this:

```cpp
struct Writer
{
  std::source_location m_l;

  Writer(std::source_location l) : m_l(l)
  {
    …
  }

  template <typename… Args>
  log(const char* s, Args… args)
  {
    …
  }
  
}

struct Logger
{
  …
  Writer operator()(std::source_location l=std::source_location::current())
  {
    return Writer(l);
  }
  …
}

// main.cpp

// writer class constructed
//   ^   writer class method invoked
//   |   ^
lgr().log(“my message %d %d”, 123, 456);
```

The idea is this:

1. Use `operator()` to instantiate a writer class, which is constructed with the correct value of `std::source_location`.
2. The new writer class then contains the necessary logging methods like `log()`, with the parameter packs we want. The file and line numbers are substituted directly from the class member variable.

This has the benefit of separating the ‘file holding’ class from the ‘log writing’ class, and allows us to fix the fight between the parameter pack and default argument order.

Is this the most efficient? Probably not - I think the compiler will elide the copy during the construction of the writer, but you would still construct on every line you log.

Is it the safest? Also probably not - in the actual code I put some safeguards like having a private constructor, but I’m sure there will be some ways that something can go wrong (and that I’ll update the code with as I find out).

But I got what I wanted: a short logging method call that is barely longer than a `printf`, and works exactly the same way.

The final code is [here](https://github.com/icyveins7/spfLogger) if you’d like to see or use it!
