---
title: "Interesting Tidbits from GTC 2025: Asynchronicity Beyond Streams"
date: 2025-03-24T20:00:00+08:00
draft: false
toc: false
images:
tags:
  - "cuda"
  - "gtc"
---

> Some notes for CUDA programmers who haven't kept up with the times; this first post covers in-kernel pipelining..

I recently had the privilege of being sponsored to attend the March 2025 GTC in person. While the sessions were very largely dominated by AI-related things, I generally selected the CUDA-related ones, since they were more relevant to my work (and interests). 

I'll try to encapsulate some of the new things I learnt while I was there into a few major concepts. Note that most, if not all of these, are not shiny new CUDA features - some of them have been out for several years, but I just didn't know about them, so I learnt about them there. 

Edit: it seems like each section is getting a bit lengthy, so I'm going to split them into separate posts. This is the first one.

# You've Heard of Async Stream Execution, Now Get Ready for Async In-Kernel Execution

Old CUDA programmers already know about asynchronicity via memory-copy overlaps, latency hiding etc. The story is pretty simple:

- Enable and use non-default streams
- Within a stream, all commands are processed serially
- Across streams, all kernels are processed in parallel (as far as the launch is concerned, execution is still up to the SMs)
- For memory copies to and from the host, you also have to make sure the memory is pinned

CUDA stream concepts are pretty old. They are analogous (in my opinion) to CPU threads, where you simply fire off a bunch of functions/instructions that you'd like to execute in parallel, and let the scheduler handle it; if there are SMs available to service your kernels, they will do so, similar to how if there are active CPU cores to execute CPU functions, they will do so.

## Bytes In-Flight and Memory Prefetching

CPUs nowadays do a pretty fantastic job of prefetching both required memory and required instructions (which has also led to hardware-level security problems but that's another story..).

Traditionally, GPUs have not done this, but we can now quite easily perform this with CUDA's new pipeline functions. A good resource is [this blog post](https://developer.nvidia.com/blog/boosting-application-performance-with-gpu-memory-prefetching/), but I'll try to summarise very quickly.

## Conceptual Understanding

The first concept to understand is what is known as _bytes in-flight_. This wasn't in the blog post link but it was mentioned many times across several sessions in GTC. The idea is that, as much as possible, you want to maximise the uptime of global memory reads or writes.

We already know that global memory access is slow, so what we want is to ensure we can do it as quickly as possible, and hide it as much as possible. The old ideas of global memory coalesced access and vectorised loads still hold true today, and contribute to this thought process.

The new idea is to then overlap memory access inside the kernel with compute. When a warp tries to access global memory (a load instruction), it takes several cycles before the values are actually ready to be operated on. If there are no other instructions that can be performed in the meantime, this results in a _stall_. Effectively, the SM _cannot proceed_ because it needs the loaded data to be ready before it can continue.

As mentioned, this isn't new to GPUs, and is why prefetching was invented for CPUs. To solve this, CUDA introduced the pipeline interfaces. Let's assume that a load instruction that we are executing requires 5 cycles to complete (latency). We can split our kernel's work into batches. For each batch with index $n$, we do the following:

1. Load the data required for batch $n+1$ asynchronously using the pipeline mechanisms. This will not block and will allow the kernel to continue executing.
2. Block/wait for data loads for batch $n$ to complete; this would have started in the previous iteration.
3. Run computations for batch $n$.

A more graphical representation might look like this:

```
Loads   :  X---- n+1 ----> X---- n+2 ----> ...
Computes:  X---- n   -->   X---- n+1 -->   ...
```

I deliberately made the computes take a shorter amount of time to demonstrate the wait on the data loading. The reverse - computes take longer than loads - still warrants this method; the idea is now we are bound by whichever operation takes longer, __but not by their sum__.

It is important to remember that all of the above is taking place _inside the kernel_; although the timeline view is similar to how one might have pipelined batches previously with compute and copy streams, this is a different, _deeper level of asynchronicity_. __Do not confuse this with streams!__

## `libcudacxx` pipelines

Since the blog post covers the C-like primitives, I'll try to cover how you would do this via the C++ APIs provided inside `libcudacxx`. See the [original API](https://nvidia.github.io/cccl/libcudacxx/extended_api/synchronization_primitives/pipeline.html?highlight=pipeline) for more details.

We'll look at the _unified_ pipeline for now - this is the simpler case where all threads are producers and consumers. You can do fancy things with the _partitioned_ pipeline where you carve out some threads to do either one, but it's probably not necessary for most cases.

The pipeline mechanism is a template construct that uses compile time constants where you define the number of stages and the scope of cooperation, similar to CUDA's cooperative groups. Let's look at a more annotated version of the example code:

```cpp
#include <cuda/pipeline>
#include <cooperative_groups.h>

template <typename T>
__global__ void example_kernel(T* global0, T* global1, cuda::std::size_t subset_count) {
	extern __shared__ T s[];
	auto group = cooperative_groups::this_thread_block();
	// Split our shared memory into 2, one for each stage
	// Here we have 2 inputs we want to load, and we load 1 element for each
	// thread, so we use 2 * group.size() for each stage
	// | stage0, input0 | stage0, input1 |
	// | stage1, input0 | stage1, input1 |
	T* shared[2] = { s, s + 2 * group.size() };

	// Create a 2-stage pipeline, synchronised at the block level.
	constexpr auto scope = cuda::thread_scope_block;
	constexpr auto stages_count = 2;
	__shared__ cuda::pipeline_shared_state<scope, stages_count> shared_state;
	auto pipeline = cuda::make_pipeline(group, &shared_state);
	
	// Prime the pipeline.
	// Here we are submitting things that we want to load asynchronously
	pipeline.producer_acquire();
	// NOTE: these functions DO NOT start the memcpy. Think of it like an
	// SQL transaction; we are just queueing the things we want to happen
	cuda::memcpy_async(group, shared[0],
					 &global0[0], sizeof(T) * group.size(), pipeline);
	cuda::memcpy_async(group, shared[0] + group.size(),
					 &global1[0], sizeof(T) * group.size(), pipeline);
	// This is what actually fires off the load instructions!
	// Think of it as now having 'Producer State: 0'
	pipeline.producer_commit();

	// Question: Why did we have to do the above?
	// Answer: same as with other pipeline mechanisms. We need to seed
	// the first batch before our iterations begin. Otherwise, the first
	// batch wouldn't be ready when we first need it in the loop.

	// Pipelined copy/compute.
	// Notice that we start on iteration 1 because that's the next batch
	// of data we are going to load
	for (cuda::std::size_t subset = 1; subset < subset_count; ++subset) {
		// The next section is identical to above;
		// we are loading the n'th batch of data
		pipeline.producer_acquire();
		cuda::memcpy_async(group, shared[subset % 2],
						   &global0[subset * group.size()],
						   sizeof(T) * group.size(), pipeline);
		cuda::memcpy_async(group, shared[subset % 2] + group.size(),
						   &global1[subset * group.size()],
						   sizeof(T) * group.size(), pipeline);
		// Again, loading doesn't start until we commit
		// In the first iteration, we would now have 'Producer State: 1'
		pipeline.producer_commit();

		// In the first iteration, at this point, we would potentially have
		// both Producer State: 0 and Producer State: 1 in flight.
		// Note that we haven't done anything with the consumer yet so
		// the consumer would be at State 0.

		// Here we are waiting on the PREVIOUS batch of data
		// In the first iteration, this would entail having the consumer
		// wait on State 0.
		pipeline.consumer_wait();
		// After this it is hanging on to State 0 while it computes.
		compute(shared[(subset - 1) % 2]);
		// And finally when we release, the consumer increments to State 1,
		// in preparation for the next iteration.
		pipeline.consumer_release();
	}
	
	// Drain the pipeline.
	// This is to compute the last iteration.
	pipeline.consumer_wait();
	compute(shared[(subset_count - 1) % 2]);
	pipeline.consumer_release();

}

```

## Notes and guidelines to follow

The above makes clear that the pipeline mechanism suffers from 1 obvious downside:

> Pipelining requires $n$ times the memory for $n$ stages.

This isn't unique to this particular mechanism; all pipeline concepts usually trade extra memory for more throughput. Note that the pipelining does not necessarily have to be done via shared memory - registers (stack variables or arrays) can also be used, but these assume that the lengths required are compile-time constants as well.

As always, premature optimization is evil and should be avoided. You can and should use Nsight Compute to check whether latency hiding within the kernel will be beneficial (and the degree to which it will be beneficial). 

> Don't do this at the beginning, but rather at the end when the kernel is already _'ready'_, because the pipeline mechanism can make things messy if the computations are not organised in a simple manner.

As a side-note, `thrust` and `cub` libraries are actually built to do this internally for the templates where it can. This is why if your operation can be pipelined and you write your own custom CUDA kernel where you _don't perform pipelining_, you will often be __slower__ than the equivalent `thrust` functor call.

> If you can use `thrust` (or `cub`) to express your computation, do so, because `thrust` will internally pipeline the kernel for you.

# For those with money: Programmatic-Dependent Launches (PDL) and Tensor Memory Accelerators (TMA)

These are both features introduced in compute capability 9.0 and above i.e. Hopper cards like H100 and newer (yes, that's the previous generation since Blackwell just released, but I don't have access to one either). Since I'm stuck in the Ampere days, I'll only touch on these 2 very briefly. The interested reader can simply search these 2 keywords for more information.
## PDL

This allows you to overlap kernels _within a stream_. Essentially, what you do is to mark points inside consequent kernels where dependence is absent. A common example is pre-loading some constant data at the start of the kernel:

```
| --- Kernel 1 --- |
             | xxx | -- Rest of kernel 2 -- |
             ^
             |
             Kernel 2 can start here to read data that doesn't
             depend on kernel 1's output. Then it can wait on
             kernel 1 to complete before it does the rest.
```

## TMA

This is somewhat like a further hardware extension of the pipeline mechanism. There is now a separate unit called the TMA that performs the data copies from global memory. There are several pros to this:

- No/less registers used; since a separate unit is doing the reads, the threads don't need to read into registers first
- Skips the entire cache hierarchy; goes straight from global to shared

But there are also cons:

- The code (in my opinion) is a lot more complex
- There are far stricter memory alignment requirements
- No option to go to registers directly (maybe not necessarily a con, depending on use-case)



