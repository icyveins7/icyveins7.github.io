---
title: "Avoid being baited by your printf statements in CUDA kernels"
date: 2026-03-23T20:00:00+08:00
draft: false
toc: false
images:
tags:
  - "c++"
  - "cuda"
  - "printf"
---

> Behaviour of printf on device is not the same as on host!

If you're a `printf` aficionado like me, then you use `printf` for debugging. A lot. In fact I previously wrote a small logger that uses `printf` called [spfLogger](https://github.com/icyveins7/spfLogger). I do enjoy the flexibility of tuning every single width/precision with a few characters, and it's something I haven't yet seen C++ be able to emulate with as little irritation.

> I haven't gotten around to writing code using newer `std::format` or `println` yet, so the jury's out on that.

But that's not the topic today. I just want to highlight something that can happen when you use `printf` in CUDA kernels, which is what I do very often.

# A simple example

Consider the following kernel and its equivalent host function:

```cpp
template <typename T, typename U>
__global__ void badprintfkernel(const T *a, const U *b) {
  // i do 4 prints here, you'll see why below
  printf("device: a = %d, b = %d, a = %d, a = %d\n", *a, *b, *a, *a);
  printf("device: a = %d, b (with ld) = %ld, a = %d\n", *a, *b, *a);
}

template <typename T, typename U> void printfhost(const T *a, const U *b) {
  printf("host: a = %d, b = %d, a = %d\n", *a, *b, *a);
  printf("host: a = %d, b (with ld) = %ld, a = %d\n", *a, *b, *a);
}
```

We invoke the kernel with 1 thread just to see the prints, and also the host function:

```cpp
thrust::device_vector<int32_t> d_32(1);
thrust::fill(d_32.begin(), d_32.end(), 0xFF332211);
thrust::device_vector<int64_t> d_64(1);
thrust::fill(d_64.begin(), d_64.end(), 0xFF332211112233FF);

thrust::host_vector<int32_t> h_32 = d_32;
thrust::host_vector<int64_t> h_64 = d_64;
badprintfkernel<<<1, 1>>>(d_32.data().get(), d_64.data().get());
printfhost(h_32.data(), h_64.data());
```

What do you expect to see?

# Not just undefined behaviour, but unexpected behaviour

This is what gets printed on the host:

```
host: a = -13426159, b = 287454207, a = -13426159
host: a = -13426159, b (with ld) = -57664913528441857, a = -13426159
```


Now this is what gets printed on the device:
```
device: a = -13426159, b = 0, a = 287454207, a = -13426159
device: a = -13426159, b (with ld) = -57664913528441857, a = -13426159
```

For reference, `0xFF332211` is -13426159 whereas `0xFF332211112233FF` is -57664913528441857.

So 2 things have happened on the device:

1. For `int64_t`, the 'cast' seems to have just completely bugged out, as it simply prints `0`.
2. It appears to have corrupted the subsequent argument's formatting. We see what we would expect to see (287454207) for `b` in the 3rd argument which prints `a`. Only on the 4th argument which prints `a` again is the printed value correct.

Now, it is understood that `printf`'s behaviour is technically undefined when the format specifier is invalid i.e. using `%d` for a 32-bit integer. However, in the CPU code, you can clearly see that what it prints is the least significant 32-bits of the number; 287454207 is `0x112233FF`, the lower 32-bits of the 64-bit number used as the input. This is in fact, what I have come to expect, which is why it can catch me off-guard when I don't pay attention to my `printf`s in kernels.

Usually, especially during development and basic unit tests, I would instantiate small numbers to test my kernels, but may possibly swap the types around to ensure things work (or to time the kernel with different types).

This quirk of `printf` in the kernel means that if I print some number the wrong way (format specifier), even without any risk of overflow, the number will be 0 (or gibberish - at least I think I've encountered gibberish before).

> For example, if the result of some number is 5, since 5 is smaller than 32-bits, it would print the same way in CPU code whether I used `%d` or `%ld` or `%lld`, since the bits 32-63 don't matter anyway.

Even worse, it will mess up other numbers in the same print statement, confusing me further, since it'll start to make me second guess the calculations involving the other numbers I am printing.

All in all, this is just a short post to warn CUDA `printf` users that you should be very careful when interpreting the results of your print statements, especially in templated scenarios where it is (or may be) difficult to change the format specifier based on the templated types. Refer to the [official docs](https://docs.nvidia.com/cuda/cuda-programming-guide/05-appendices/cpp-language-support.html#printf) for other limitations, like the maximum of 32 printed arguments (which I have discovered for myself before).
