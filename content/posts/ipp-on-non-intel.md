---
title: "Getting IPP to work on non-Intel chips"
date: 2024-02-19T20:00:00+08:00
draft: false
toc: false
images:
tags:
  - "ipp"
  - "c++"
---

> Intel Performance Primitives is not guaranteed to work on non-Intel chips, but there are some ways around it..

Over the last year or so I've written a C++ wrapper of header-only templates around a library known as Intel Performance Primitives (IPP). I often use this for its signal processing library, which is - at least by my measurements - one of the fastest, if not the fastest one around. It also has the benefit of having almost everything I need in one place: FFT/DFTs, math array processing, low-pass filtering etc. You can see my templates at the [`ipp_ext`](https://github.com/icyveins7/ipp_ext) repository.

Intel themselves make no promises about IPP's performance on non-Intel chipsets; in fact, they recently pulled support for MacOS, since Apple has gone with Apple Silicon now.

This is trouble for people like me who develop ___for___ Intel chipsets, but ___on___ non-Intel chipsets (like my home computer and my Macbook).

# Dynamic Linking is the Problem 

On Apple Silicon, any linkage with IPP will refuse to build when dynamically linked. Only __static linking__ works; that is, you link with the `.a` libraries directly. This still throws a whole shitload of warnings, but __at least it builds__. I did this when writing my Catch2 tests within `ipp_ext`.

On AMD however, the problem is more nefarious. I recently spent the better part of a week trying to debug an unknown issue with some code that uses IPP; it would randomly crash when referencing a DFT function, and explicitly checking every reference and pointer would prove that everything was okay. I opened it in Visual Studio debugger to really make sure everything was okay i.e. not that my copy/assignment methods were failing etc.

It seemed likely to me that the problem was the function itself was problematic. IPP dispatches the correct function that corresponds to your chip's instruction set at runtime, so maybe on non-Intel chips this fails for the dynamically linked libraries?

So I just relinked with the static libraries (my AMD is on Windows) which are `ippcoremt.lib`, `ippsmt.lib` and `ippvmmt.lib`. The crashes stopped.

Why? I have no idea, and IPP is closed-source so I will never know.

# Implications

I hope Intel doesn't find this and then decide 'oh shit he found a loophole'. You have to realise that we use this because we want to deploy on Intel eventually.

For the rest of us using IPP, remember that ___static linking is your friend___ if all other errors have been checked and you're developing on a non-Intel chip. __Do it even if it compiles properly__, because the runtime crash is almost impossible to debug!

# For the nay-sayers

I suspect someone who reads this is going to think: nah bro you definitely wrote some memory leak in, or something along those lines.

Please, if you manage to not crash ___consistently___ when linking with IPP dynamically, do let me know how. The setup is here at my non-master [`pybindCZTdebug` branch of `pydsproutines`](https://github.com/icyveins7/pydsproutines/tree/pybindCZTdebug/pybinds/ippCZT/testCZT). I link with my `ipp_ext` repository there so maybe the error is inside `ipp_ext`? But I've never found the error (and why would the error only surface when dynamically linking..).

Build the solution and link with dynamic libraries - `ippcore.lib`, `ipps.lib` - and issue a fix for me if you can find it. I tested this on my AMD Ryzen 5600X (which crashed almost every time, but not every time mind you) and another 2 Intel-based chips (which never crashed with the dynamic linking).

