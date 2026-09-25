---
title: "Occupancy-maxxing is just Starcraft"
date: 2026-08-12T20:00:00+08:00
draft: false
toc: false
images:
tags:
  - "occupancy"
  - "starcraft"
  - "cuda"
---

> If you can manage minerals and gas, you can manage CUDA threads and registers.

> CUDA is just resource management.

Okay, of course there's a lot more to squeezing CUDA kernel performance than *just resource management*, but occupancy is very often the simplest thing to strive for, once you know how. And usually, if you've hit 100% occupancy, then unless you've done something heinous in your code, further optimizations are unlikely to budge your performance by large factors (aside from an entire algorithmic shift, but we won't discuss that here).

# Starcraft and unit compositions

For those who are too uncultured to have played Starcraft before, the goal of the game is essentially to construct a base, gather resources, and then build an assortment of units to destroy your enemy.

> I'm going to use Protoss units and buildings here, because *Protoss master race*. Also because **pylon** sounds better than **bunker**(boring) and **overlord**(ew). Facts, not biased at all.

There's a few things to keep in mind during a game:

- **Supply**: this is the maximum *army count* you can have. You increase this by *building more pylons*.
- **Minerals**: this is the base resource. It's generally the more common resource, and you have more of it than gas.
- **Vespene gas**: this is the secondary resource. It's harder to get, but is used to build more *advanced units*.

Imagine you have some limited amount of the above 3 at some point in the game. Your goal is to maximize the use of your resources. Let's say you have

- 16 supply
- 400 gas
- 1500 minerals

We'll choose from a small subset of possible units to make it simple:

| Unit Name | Mineral Cost | Gas Cost | Supply Cost |
| :--- | :--- | :--- | :--- |
| **Zealot** | 100 | 0 | 2 |
| **Stalker** | 125 | 50 | 2 |
| **Immortal** | 275 | 100 | 4 |
| **Colossus** | 300 | 200 | 6 |
| **Carrier** | 350 | 250 | 6 |

You could say: I just want carriers. So you derp your way over to your Stargate and just build one... and then you run out of gas. Obviously, this is a terrible army composition. You still have a lot of supply - 6/16 - along with a ton of minerals and gas left over.

Alright, so then you just want to make sure you at least use all your supply. Simple way is to mineral dump into zealots, so you max out with 5 more zealots. Total consumption is 350 + 5 * 100 = 850 minerals, and 250 gas. Obviously, this is still leaving a lot on the table.

Consider getting 2 stalkers, a colossus, an immortal and a zealot. You now use 2 * 125 + 300 + 275 + 100 = 925 minerals, and all 400 gas. This is a far better use of all your resources.

# What (theoretical) occupancy is

The idea in GPU kernel occupancy is basically the same. You have the following to play with:

- Number of threads per SM
- Number of blocks per SM
- Number of registers per SM
- Shared memory per SM

https://docs.nvidia.com/cuda/cuda-programming-guide/05-appendices/compute-capabilities.html#features-and-technical-specifications

Technically there's a list of things, but honestly you can just focus on these few, because they're the ones that come up the most often.

The easiest one to think about is number of threads per SM. Let's consider a typical CC 86 card with 1536 threads per SM. The idea is that you essentially slot your blocks onto the SM one at a time, until the SM has insufficient resources to slot in the next block.

A simple example is the common novice GPU programmer gotcha of launching very large blocks; usually the maximum block size is 1024 threads. With 1536 threads per SM, the SM cannot fit in the second block after it already accepts one. SMs do not accept fractions of blocks. This essentially means that you *might* have 1/3 of the compute resources sitting unused; `nsys` will tell you your theoretical occupancy is 66%. If there is no strict reason for needing 1024 threads per block, then you would gain this compute back simply by lowering the block size.

Occupancy is thus a statement of how many threads you are using per SM.

The converse is also true. There is a maximum number of blocks that can be in flight per SM. That number usually corresponds to something less than 128 threads per block. This is one reason why many CUDA tutorials recommend starting with 128 thread blocks..

So for our CC 86 card with 1536 threads per SM that is 16 blocks. If you were to launch your kernel with 32-thread blocks, the SM *will still only hold up to 16 blocks*. Thus that means the total number of threads being handled per SM falls to a measly 16*32=512 threads, and your theoretical occupancy will rest at 33%. 

# Adjusting your CUDA army composition

In all the scenarios above, what we are really trying to do is to simply *ensure the SM is fully saturated with threads*. This is very easy to do with simple kernels; just change your block size to a number like 128 or 256, in most cases.

Two things to address here:

1. **Why does this even matter?** Performance. Internally, although the SM is likely not processing all its threads concurrently, using more threads here allows it to hide latency between memory, compute and whatever else. I agree that this isn't a convincing argument, so the best way to know is to try it yourself whenever possible.
2. **What if changing the block size isn't doing anything?** Then you are likely dealing with the next 2: registers and shared memory. We discuss this now.

# The argument against fat kernels

You have probably heard of kernel fusion. The primary reason for this is to reduce global memory pressure by doing more work within one kernel; that way, you don't have to read and write back to global memory at what used to be the beginning/end of 2 separate kernels.

Now, I would go so far as to claim that this is essentially true all of the time. You should default to this mindset when looking for optimization avenues; however, we should always remain cognizant of what we give up by doing this. This is why I tend to write small kernels and then merge them, rather than write long ones and split them later. It's always far more maddening to try to disentangle a fat kernel.

So why fatshame kernels? Well, in general, the longer your kernel is, the more registers per thread it will use. The compiler does an excellent job of reducing and reusing registers, but it isn't perfect. It is unlikely that any non-trivial kernel will stay below 20 registers per thread.

The story here is the same as that of the thread count. If we exceed the total number of registers per SM, the SM will cut blocks until it fits. Consider the following configuration:

- Threads per block: 128
- Registers per thread: 50
- Maximum registers per SM: 65536
- Maximum threads per SM: 1536
- Maximum blocks per SM: 16

As we saw earlier, under normal circumstances, you would easily fit 12 blocks ($12 \times 128 = 1536$) worth of threads on a single SM. But now the SM needs to ensure all the registers your blocks need are available. This comes out to $128 \times 50 \times 12 = 76800$, which exceeds the 65536 cap. So what happens? The SM simply refuses to house blocks that would exceed its registers, so it really only houses $65536/(128\times50) = 10.24$ blocks. No such thing as a fractional block, so we get 10 blocks in flight per SM. That's a theoretical occupancy of $\frac{10}{16} = 60\%$.

> In fact, the problem is actually worse than this. CUDA actually allocates registers at a warp granularity. In most cases, this is 256 registers per warp (or 8 registers per thread). What this means is you get *breakpoints*; 40 registers per thread and 41 registers per thread is a gigantic jump, since 41 registers per thread is effectively 'the same as 48 registers per thread'.

The same goes for shared memory - static and dynamic alike. You may have heard that the maximum shared memory per thread block is 64KB (at least by default, yes I know you can increase it nowadays, but the point still stands).
You might then think it's ok to use as much of it as possible for each thread block; after all, we get to exploit the fast cache-like memory bandwidth right? Not quite. You have essentially the same maximum shared memory per block as maximum shared memory per SM. So if you use all 64KB of shared memory for 1 block, then the SM will have capacity to hold *only that one block*. If your block size is 128 threads, then your theoretical occupancy is tanking all the way down to $\frac{128}{1536}$ (or a similar calculation for your compute capability). That is an 8% to be ashamed of.

# Occupancy isn't everything

After reading all this, it might sound like occupancy is the only thing you should care about. But in reality, this is usually not the case. Kernels are complicated, and just because you increase occupancy does *not* guarantee better performance.

The tradeoffs are the same:

- If you cut registers you usually end up doing more computations
- If you cut shared memory you either store/load more from global memory, or you use more registers

And these are just the technical reasons. You also often lose code clarity if you force your kernel to do awkward computations just to reduce some shared memory workspace or registers. Sadly, there is no magic bullet here; at the end of the day the correct thing to do is to profile your choices.

> Please profile your damn choices. With AI nowadays there's no reason not to try a bajillion different kernel configurations and just pick the best one.

