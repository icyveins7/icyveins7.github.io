---
title: "Gotta go (randomly) fast, thrust vs cuRAND"
date: 2025-03-10T20:00:00+08:00
draft: false
toc: false
images:
tags:
  - "c++"
  - "c"
  - "cuda"
  - "rng"
  - "thrust"
  - "curand"
---

> How fast can you generate (pseudo-)random numbers on the GPU?
> How fast can you generate (pseudo-)random numbers on the GPU?

I recently had to answer this question. Well, not exactly this question, but this was a key part of it.

A quick google search will bring up two common methods (libraries) when trying to do this in CUDA: thrust and cuRAND. Thrust is known to be a lot easier to set up; no need to write the kernel code and nitty gritty details, so I started with that.

Before I begin though, [here's a link to my growing GPU-related codebase](https://github.com/icyveins7/gpu_benchmarks). Some of the following snippets come from there.
# The `thrust` way

Generating random numbers in `thrust` is somewhat similar to how you would do so in `std` C++.

1. Initialize an RNG engine.
2. Initialize a distribution, like `uniform_real_distribution`.
3. Call the distribution on the engine via `operator()` to generate a random number.

This is usually done, as seen in the examples, via a custom `struct` with an overloaded `operator()`, which is passed to a `thrust::generate` call, like so:

```cpp
// The following is adapted from
// cuda-samples/Samples/3_CUDA_Features/cdpQuadtree/cdpQuadtree.cu
template <typename Engine = thrust::random::default_random_engine>
struct Random_generator2d {

  int count;
  __host__ __device__ Random_generator2d() : count(0) {}
  __host__ __device__ unsigned int hash(unsigned int a) {
    a = (a + 0x7ed55d16) + (a << 12);
    a = (a ^ 0xc761c23c) ^ (a >> 19);
    a = (a + 0x165667b1) + (a << 5);
    a = (a + 0xd3a2646c) ^ (a << 9);
    a = (a + 0xfd7046c5) + (a << 3);
    a = (a ^ 0xb55a4f09) ^ (a >> 16);
    return a;
  }

  __host__ __device__ __forceinline__ thrust::tuple<float, float> operator()() {
    unsigned seed = hash(blockIdx.x * blockDim.x + threadIdx.x + count);
    // thrust::generate may call operator() more than once per thread.
    // Hence, increment count by grid size to ensure uniqueness of seed
    count += blockDim.x * gridDim.x;

    Engine rng(seed);
    thrust::random::uniform_real_distribution<float> distrib;

    return thrust::make_tuple(distrib(rng), distrib(rng));
  }
};

Random_generator2d<thrust::random::default_random_engine> rnd;
thrust::generate(
    thrust::make_zip_iterator(
        thrust::make_tuple(d_x.begin(), d_y.begin())),
    thrust::make_zip_iterator(thrust::make_tuple(d_x.end(), d_y.end())),
    rnd);
    
```

In the above example, the generator functor has been modified to output 2 random values per call. There is also a custom hash function in the sample.

This is all well and good, but I had several issues with this:

- `thrust` documentation is nearly non-existent. I had to search for stack overflow articles and look at the samples directly to really figure out what to do.
- Because of the above, I wasn’t really sure whether I could remove the engine instantiation from within the functor `operator()`. Not being able to do so would imply that every time the functor is called, it would restart from the beginning i.e you wouldn’t be able to use this in a loop, since the 2nd call would generate the same numbers as the 1st call. *It could probably be done - by returning the engine and taking it in as an input? - but there was no reason to experiment since cuRAND already had this well-documented*.
- Like all other `thrust` things, there was no way to finely control the grid and block dimensions.
- The `thrust::generate` and functor way of doing things, while simple if trivially generating straightforward data structures, started looking uglier - in my opinion - when more complicated indexing was required.

# This (cuRAND) is the way

The NVIDIA mandalorians have declared it so.

Seriously though, it seems pretty obvious that NVIDIA *wants* you to use cuRAND for HPC implementations. [And the performance looks pretty sick to be honest.](https://developer.nvidia.com/curand)

The library by design forces you to initialise a bunch of RNG states with `curand_init`. Each of these states are used in function calls like `curand_uniform` to generate random numbers, after which the states are updated so that the next call will generate the next random number in the sequence.

The beauty of this is that there is a lot of choice for how to use the states:

1. The most basic way: use one state per thread, per element. You'd save the updated states back into global memory.
2. A bit more complex: use one state per thread, per N elements. You'd load the states from global memory, generate N elements, then write back to global memory at the end.

You also get to tweak the index to start at in the subsequence, just from either the `curand_init` arguments (subsequence is usually the thread index, offset is what you're looking for), or via a `skipahead` function call (which would be the same as if you had init-ed at that offset, but you don't want to init again).

Since all of these calls are done using the device API - inside a normal kernel function - you get to play around with all the standard block/grid choices you would with a kernel as well; this is unlike in `thrust` functor calls where the grid and block dimensions are chosen for you.

 I ended up writing a simple class to wrap these operations, which could be easily extended for other use-cases, either by explicit inheritance or just a simple rewrite:

1.  Class constructor initialises the number of states; how the states are used is internal to the implementation, but what is important is that it is constant i.e. used in the same way for every invocation later. This essentially allocates device memory and performs the `curand_init` calls appropriately.
2. Simple public methods to call the kernel that does the RNG (and any other pre/post-computations). Again, this is implicitly understood to be constant every call (the shape of the data, and the grid/block dimensions operating on it are the same and part of the design for that use-case).
3. Skipahead function calls are also defined, similar to the previous point.
4. Destructor doesn't need to be defined since RAII `thrust` vectors are used to store the states.

You can take a look at some of the test implementations I made [here](https://github.com/icyveins7/gpu_benchmarks/blob/master/proj_rng/curand_extensions.h) and [here](https://github.com/icyveins7/gpu_benchmarks/blob/master/proj_rng/curandrng.cu). I originally wanted to try out `curand`'s float4 generators that [only work with Philox](https://docs.nvidia.com/cuda/curand/group__DEVICE.html#group__DEVICE_1ge214aeb22edf5f523258487f6a8ae78b), but I didn't bother because this part didn't end up being a bottleneck in my final code.
