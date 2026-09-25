---
title: "Quickly sketching out a BlockQuickSelect"
date: 2025-07-20T20:00:00+08:00
draft: false
toc: false
images:
tags:
  - "cuda"
  - "quickselect"
  - "cub"
---

> I wrote a blockwide quickselect. That's the post.


As of today, NVIDIA's `cub` library has a few block-wide primitives to do sorting - like `BlockRadixSort` - but none that do the equivalent for the $k$'th order statistic i.e the $k$-th smallest element.

This would be functionally equivalent to the CPU's `std::nth_element`, but that [apparently uses Introselect](https://en.cppreference.com/w/cpp/algorithm/nth_element.html), which is a tad too much for me to want to implement. I'm going to stick with the simpler [quickselect](https://en.m.wikipedia.org/wiki/Quickselect).

# What we need

Similar to `cub`'s block-wide primitives, we probably want to just get some shared memory and work on the data inside it. I'm at a crossroads here: using statically allocated shared memory like `cub` via templates makes it easy to implement, but doesn't let you handle differing sizes per block (at least not explicitly).

But dynamic allocation may take a bit more time to implement?

~~Meh, let's just go with static for now.~~ I ended up doing some half baked static and dynamic nonsense, but whatever.

# The flow

1. Load block's array from global to shared memory.
2. Sync threads.
3. Select pivot; just use the first element(?)
4. All threads other than pivot compare against pivot.
5. Warp-wide counters for `< pivot` and `> pivot` get warp-aggregated atomically added to the two counters in shared memory.
6. The previous warp aggregated counters let us update a second workspace with the left-side and right-side elements. The way I thought to do this would basically be writing all left-side elements from index 0, forwards. Right-side elements would then be written from index N-1 backwards.
```
# example input
4 5 6 3 2
|
pivot

# example output
3 2 X 6 5
    |
    not used
```
7. Sync threads.
8. Check counters and find which side to recurse into, or end if the pivot is the answer.
9. Go back to 3.

Sounds easy enough. I'm sure I can do this in ~~half an hour~~ ~~1 hour~~ 2 hours, right?

## Why not just swap rather than use a second workspace?

Because it's easy. Honestly, maybe doing the swaps would be faster *and* more memory-efficient, but this was quicker to implement (read the title).

Here I just allocate enough shared memory for two workspaces of equal length, and swap them after every iteration; no need to care about handling races across the blocks.

# How fast is it?

As it turns out, for my test with 10000 rows of randomized lengths (up to a max of 100), with each block taking one row, this was about 50 % faster (33% reduction in time) than `cub::BlockRadixSort`! I compared it to selecting the median element here (which is what I was concerned with):

```
 ** CUDA GPU Kernel Summary (cuda_gpu_kern_sum):

 Time (%)  Total Time (ns)  Instances  Avg (ns)  Med (ns)  Min (ns)  Max (ns)  StdDev (ns)                                                  Name
 --------  ---------------  ---------  --------  --------  --------  --------  -----------  ----------------------------------------------------------------------------------------------------
     59.2            62336          1   62336.0   62336.0     62336     62336          0.0  void blockwise_median_kernel<unsigned short, (int)128, (int)1, (bool)1>(const T1 *, int, int, const.
     38.0            40000          1   40000.0   40000.0     40000     40000          0.0  void blockwise_quickselect_kernel<unsigned short>(const T1 *, int, int, int *, int *, T1 *)
```
*Above timings from my 5080. Also, what a timing - exactly 40000 lol.*

Not too bad for 2 hours of work I guess? Code is [here](https://github.com/icyveins7/gpu_benchmarks/blob/master/proj_median/median.cuh) for those who want to `nsys profile` it yourself on your GPU. *And check out the rest of my repo while you're at it!*

The struct works fairly similarly to the `BlockRadixSort`, in the sense that you instantiate it at the thread level and then pass it some shared memory. Everything else is taken care of inside the `__device__` methods, and the result should be valid for the first thread, as seen in the example.

Like previously mentioned, it assumes a maximum length for a row input, but only utilises a row-specific length when performing the calculation.
