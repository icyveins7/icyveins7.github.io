---
title: "Iterative algorithms with CUDA"
date: 2025-11-29T20:00:00+08:00
draft: false
toc: false
images:
tags:
  - "c++"
  - "iterative"
  - "cuda"
---

> Because not everything can be made un-iterative..

There's a pretty large class of algorithms that have been developed for CPUs that involve, optimally, an iterative solver.

Usually, this is defined by some notion of *convergence*; some variable is updated, and then at the end of each iteration it is checked to determine whether further iterations are required.

This is all fine and dandy in CPU-land, but in GPUs this almost always makes the flow awkward.

# The copy-back

The easiest way to do this is usually a `memcpy` back to the host. We can make this as efficient as possible by of course making the memory pinned (it's usually just a single variable after all).

So the flow would be something like

1. Launch iteration kernel.
2. Pull `check` back to host pinned memory.
3. Decide if iterations should continue.
4. Return to 1, or break.

If your iteration kernel takes fairly long, the pull back and CPU-side decision making probably be insignificant in comparison, so this method is actually decent.

# The cooperative grid

However, there is another way to do this; if anything, the aspiring CUDA practitioner should know of all the ways, just for the sake of it.

This is via in-kernel [grid synchronization](https://docs.nvidia.com/cuda/cuda-c-programming-guide/#grid-synchronization). In essence, this is just another flavour of cooperative groups, but grid-wide instead of the more commonly known block-wide groups.

The idea is pretty simple, as shown in the link; slap on a `grid.sync()` at the end of a single iteration, and it would be *just like the kernel had returned*.

So the order of operations inside a cooperative kernel would now look like

1. Declare the cooperative grid.
2. Place all your previous iteration kernel's code into a `while` or `for` loop, depending on whether you want to limit the maximum number of iterations (always a good idea).
3. At the end of the *iteration* `while`/`for` loop, issue a `grid.sync()`.
4. Now the line after this is guaranteed to have every thread in the grid arrive at the same time; hence all threads of the kernel can read the `check` variable together, without any race conditions.
5. Just like before, each thread will decide if the iterations should continue (and they will all make the same decision, by definition).
6. Break and exit, or go again.

Now there is a major caveat to this, as also shown in the link: you cannot allocate a grid larger than the device (all SMs) can support at any given point. Ideally, this shouldn't matter *too much*, since if every thread in every SM is doing work then you aren't really going to get extra performance out of using a larger grid.

But ideal isn't the world we live in, and it very likely isn't the world your code lives in either. In reality, some threads may be doing nothing due to conditionals, and/or having more blocks waiting could hide some latency. This would imply that using more blocks (like allocating enough to cover all your data) can also be beneficial.

**As always, profile your own kernel**; for my recent use-case, I found that it could be slower in some scenarios and similar in others (but never faster, at least not tangibly).

