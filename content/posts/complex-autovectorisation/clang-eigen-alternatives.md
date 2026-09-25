---
title: "Clang and Eigen's alternatives to complex multiplication SIMD"
date: 2024-03-11T20:00:00+08:00
draft: false
toc: false
images:
tags:
  - "assembly"
  - "c++"
  - "avx"
  - "simd"
  - "intrinsics"
  - "clang"
  - "eigen"
---

> Clang isn't much better than MSVC for complex number multiplication, while Eigen is equivalent to GCC but uses slightly different instructions.

# Complex Number Multiplication Is Probably Not Common Enough

I think I might have bashed MSVC too much on the last post; trying to vectorise our simple 4-element complex number multiplication using clang with AVX instructions delivers [similarly poor results](https://godbolt.org/z/35dGqTfKv):

```
#include <complex>

void cmul(
    const std::complex<float>* __restrict__ x,
    const std::complex<float>* __restrict__ y,
    std::complex<float>* __restrict__ z
){
    for (int i = 0; i < 4; ++i)
        z[i] = x[i] * y[i];
}

void mul(
    const float* __restrict__ x,
    const float* __restrict__ y,
    float* __restrict__ z
){
    for (int i = 0; i < 8; ++i)
        z[i] = x[i] * y[i];
}
```

```
cmul(std::complex<float> const*, std::complex<float> const*, std::complex<float>*):          # @cmul(std::complex<float> const*, std::complex<float> const*, std::complex<float>*)
        vmovsd  xmm0, qword ptr [rdi]           # xmm0 = mem[0],zero
        vmovsd  xmm1, qword ptr [rsi]           # xmm1 = mem[0],zero
        vbroadcastss    xmm2, xmm0
        vmovshdup       xmm0, xmm0              # xmm0 = xmm0[1,1,3,3]
        vshufps xmm3, xmm1, xmm1, 225           # xmm3 = xmm1[1,0,2,3]
        vmulps  xmm0, xmm3, xmm0
        vfmaddsub231ps  xmm0, xmm1, xmm2        # xmm0 = (xmm1 * xmm2) +/- xmm0
        vmovlps qword ptr [rdx], xmm0
        vmovsd  xmm0, qword ptr [rdi + 8]       # xmm0 = mem[0],zero
        vmovsd  xmm1, qword ptr [rsi + 8]       # xmm1 = mem[0],zero
        vbroadcastss    xmm2, xmm0
        vmovshdup       xmm0, xmm0              # xmm0 = xmm0[1,1,3,3]
        vshufps xmm3, xmm1, xmm1, 225           # xmm3 = xmm1[1,0,2,3]
        vmulps  xmm0, xmm3, xmm0
        vfmaddsub231ps  xmm0, xmm1, xmm2        # xmm0 = (xmm1 * xmm2) +/- xmm0
        vmovlps qword ptr [rdx + 8], xmm0
        vmovsd  xmm0, qword ptr [rdi + 16]      # xmm0 = mem[0],zero
        vmovsd  xmm1, qword ptr [rsi + 16]      # xmm1 = mem[0],zero
        vbroadcastss    xmm2, xmm0
        vmovshdup       xmm0, xmm0              # xmm0 = xmm0[1,1,3,3]
        vshufps xmm3, xmm1, xmm1, 225           # xmm3 = xmm1[1,0,2,3]
        vmulps  xmm0, xmm3, xmm0
        vfmaddsub231ps  xmm0, xmm1, xmm2        # xmm0 = (xmm1 * xmm2) +/- xmm0
        vmovlps qword ptr [rdx + 16], xmm0
        vmovsd  xmm0, qword ptr [rdi + 24]      # xmm0 = mem[0],zero
        vmovsd  xmm1, qword ptr [rsi + 24]      # xmm1 = mem[0],zero
        vbroadcastss    xmm2, xmm0
        vmovshdup       xmm0, xmm0              # xmm0 = xmm0[1,1,3,3]
        vshufps xmm3, xmm1, xmm1, 225           # xmm3 = xmm1[1,0,2,3]
        vmulps  xmm0, xmm3, xmm0
        vfmaddsub231ps  xmm0, xmm1, xmm2        # xmm0 = (xmm1 * xmm2) +/- xmm0
        vmovlps qword ptr [rdx + 24], xmm0
        ret
mul(float const*, float const*, float*):                         # @mul(float const*, float const*, float*)
        vmovups ymm0, ymmword ptr [rsi]
        vmulps  ymm0, ymm0, ymmword ptr [rdi]
        vmovups ymmword ptr [rdx], ymm0
        vzeroupper
        ret
```

Compiled with clang 18.1.0, with ```-O3 -ffast-math -march=x86-64-v3```, this still refuses to vectorise the ```cmul``` function correctly, only using the ```xmm``` registers. I included the normal real-valued float multiplication to check that clang is indeed able to vectorise that. Note that you still need the ```__restrict__``` keywords for the vectorisation to work. Using ```-ffast-math``` doesn't seem to do anything for us in the real float vectorisation, but it does make the complex-valued vectorisation less verbose.

There's probably not enough complex number math code out there in the wild, and so only the most used compilers - like GCC - actually have decent vectorisation when it comes to this.

# How does Eigen do it?

Since we're on the topic, I decided to try and see what Eigen's implementation looks like. After all - and I just discovered yet another amazing feature here - godbolt.org actually allows you to include popular libraries, right there in the online interface!

So we have the [following, compiled with GCC](https://godbolt.org/z/rcPn8Wf9a):

```
#include <Eigen/Dense>
#include <complex>


void eigCmul4(
    const Eigen::Array4cf& x,
    const Eigen::Array4cf& y,
    Eigen::Array4cf &z
){
    z = x * y;
}

void cmul4(
    const std::complex<float> * __restrict__ x,
    const std::complex<float> * __restrict__ y,
    std::complex<float> * __restrict__ z
){
    for (int i = 0; i < 4; ++i)
        z[i] = x[i] * y[i];
}
```

We use ```Array``` here instead of ```Vector``` or ```Matrix``` since we want the element-wise products.

The assembly looks like this:

```
eigCmul4(Eigen::Array<std::complex<float>, 4, 1, 0, 4, 1> const&, Eigen::Array<std::complex<float>, 4, 1, 0, 4, 1> const&, Eigen::Array<std::complex<float>, 4, 1, 0, 4, 1>&):
        vmovaps ymm1, YMMWORD PTR [rdi]
        vmovaps ymm0, YMMWORD PTR [rsi]
        vmovsldup       ymm3, ymm1
        vmovshdup       ymm1, ymm1
        vpermilps       ymm2, ymm0, 177
        vmulps  ymm1, ymm2, ymm1
        vmulps  ymm0, ymm0, ymm3
        vaddsubps       ymm0, ymm0, ymm1
        vmovaps YMMWORD PTR [rdx], ymm0
        vzeroupper
        ret

cmul4(std::complex<float> const*, std::complex<float> const*, std::complex<float>*):
        vmovups ymm0, YMMWORD PTR [rdi]
        vmovups ymm3, YMMWORD PTR [rsi]
        vpermilps       ymm1, ymm0, 160
        vpermilps       ymm2, ymm3, 177
        vpermilps       ymm0, ymm0, 245
        vmulps  ymm0, ymm2, ymm0
        vfmaddsub231ps  ymm0, ymm1, ymm3
        vmovups YMMWORD PTR [rdx], ymm0
        vzeroupper
        ret

```

Aside from the aligned vs non-aligned ```mov``` instructions, the main difference is that Eigen uses ```vmovsldup``` and ```vmovshdup``` instead of ```vpermilps``` for two of the instructions. However, it turns out this choice is insignificant, as the same number of cycles are used for permute and shuffle:

```
Timeline view:
                    012
Index     0123456789   

[0,0]     DeER .    . .   vmovsldup     ymm3, ymm1
[0,1]     DeER .    . .   vmovshdup     ymm1, ymm1
[0,2]     D=eER.    . .   vpermilps     ymm2, ymm0, 177
[0,3]     D==eeeeER . .   vmulps        ymm1, ymm2, ymm1
[0,4]     D=eeeeE-R . .   vmulps        ymm0, ymm0, ymm3
[0,5]     D======eeeeER   vaddsubps     ymm0, ymm0, ymm1

Timeline view:
                    01
Index     0123456789  

[0,0]     DeER .    ..   vpermilps      ymm1, ymm0, 160
[0,1]     DeER .    ..   vpermilps      ymm2, ymm3, 177
[0,2]     D=eER.    ..   vpermilps      ymm0, ymm0, 245
[0,3]     D==eeeER  ..   vmulps ymm0, ymm2, ymm0
[0,4]     D=====eeeeER   vfmaddsub231ps ymm0, ymm1, ymm3
```

# FMA instructions and using ```-march=x86-64-v3```

If you noticed, the GCC code this time is slightly different from the last post. That's because I discovered that it's a lot easier to specify the entire instruction _set_ using ```-march=x86-64-v3```, which includes all of the following, taken from the [wikipedia page](https://en.wikipedia.org/wiki/X86-64): AVX, AVX2, BMI1, BMI2, F16C, FMA, LZCNT, MOVBE, OSXSAVE.

Honestly, don't know what half of them are right now, but hey, FMA and AVX are both in there and are relevant to what I do.

Surprisingly though, it seems like there's very little real difference in terms of number of cycles (only 1 less, since we have to wait for the previous ```mul``` instructions to finish), so I guess the benefit for this case is primarily in higher precision using FMA.

