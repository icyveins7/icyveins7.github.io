---
title: "Your left-right boundary search is actually a CUDA-friendly single scan"
date: 2026-06-26T20:00:00+08:00
draft: false
toc: false
images:
tags:
  - "cuda"
  - "cub"
  - "scans"
---

> At least, most of the time it is. As long as it fits the 1-2-1 pattern..

Many segments can be boiled down to what I call a 1-2-1 pattern. For an array of input elements (I'll just call them pixels, it's shorter for me to type), a naive segment detection is to loop forwards until you hit an *activation* pixel. This can internally be any condition you'd like; that is, any `if (...)` is possible here as long as you operate only on the pixel you're currently at. For simplicity, I'll assume this is the `2` pixel:

```
. . . . . 2
---------> activated
```

The activation pixel is what defines us to be within a segment. This can happen several times within a segment, but it doesn't matter (activating an already active segment does nothing):

```
. . . . . 2 2
-----------> still same segment
```

We now seek the boundaries of the segment, $[a,b]$ i.e. the (inclusive) pixels on the left and right where the segment is deemed to start/end. A naive next step is then to iterate from the first activation pixel leftwards and rightwards to look for the boundary conditions. Again, this can be defined by any `if(...)` statement, but here we will simplify it to be a `1` pixel:

```
. . 1 . . 2 2 . . 1 .
     <---- ------>
```

In this simplified scheme, anything other than 1 and 2 for the pixels are irrelevant (hence the 1-2-1 naming).

Note that under the above definition, a simple 1-1 boundary does not denote a segment, as it lacks an activation `2` pixel:

```
. . 1 . . . . 1
This is not a segment
```

# Scan when there are many

Now the astute leetcoder will probably realise that this is terribly inefficient. If you have multiple segments, you will end up doing a lot of backtracking. The fundamental idea of using a scan here is to *remember the last boundary*. In this case, we also need to remember if we are in an *active* segment.

> For the uninformed, a *prefix scan* is essentially a rolling operation with a binary functor. That is, you define an operation taking in two values - before, after - and return a new value. This new value is used in the next operation (instead of the original input). The most common variant is arguably a prefix sum, otherwise known as a cumulative sum or `cumsum` in many languages.

This easily boils down to a few simple operations:

- **Reset**: whenever we see a `1`, it means the current segment (active or not) has ended, and this is also the start of a new segment.
- **Extend**: in any other case, we simply copy the index we currently have. Importantly, we also need to keep track of the last `2` we saw, so we know at the end of a segment whether it was activated or not.

In practice, we can couple the incrementing index with the original pixels, and perform the scan in the following way:

```
 In 0 1 0 1 2 0 1 0
Idx 0 1 2 3 4 5 6 7

Sdx - 1 - 3 - - 6 -
```

Here, `Sdx` is simply a transform of the original incrementing index: set to -1 if it's not a boundary `1`,  otherwise keep the original index. Then, our scan operation looks something like this:

```cpp
FlagAndIndex operator()(
	FlagAndIndex a,
	FlagAndIndex b
){
	if (b.flag == 1) // reset
		return b; // we 'forget' everything before
	else // extend
		// copy ('drag') the earlier index,
		// take the larger flag (so we keep 'activations')
		return {max(a.flag, b.flag), a.idx};
}
```

The scan then results in this:

```
 In 0 1 0 1 2 0 1 0
Sdx - 1 - 3 - - 6 -

Out 0 1 1 1 2 2 1 1
Odx - 1 1 3 3 3 6 6
```

We can walk through this simple example:

1. Index 0. We are performing an *inclusive* scan so the first input is just copied. No-op.
2. We hit the first `1` at index 1. This hits our **Reset** condition so output becomes the current index i.e., 1, and we use the input flag `1`.
3. Index 2: **Extend**. Simply copy the same value from before; `max(flagA, flagB)` is still just `1`.
4. Index 3 has hit our **Reset** again. We hence update to the current index, 3, and flag, `1`.
5. Index 4: **Extend**. This hits the first activation. The index is dragged up (still 3) but now the flag after the `max()` operation is `2`.
6. Index 5 is again an **Extend**. Everything is copied, flag is still `2`.
7. Index 6 hits a **Reset**. The output index is updated to 6, and our flag is reset to `1`.

Hopefully this toy example makes it clear how a simple custom functor with a few operations maintains the one-pass scan.

# Okay, but where's the bounds?

Readouts should now be pretty obvious. Note that you can actually do this *during* the scan, but for now I will just explain it with another pass.

We simply need to read the inputs for `1`s, and then check the activation flag one step on the left. If it's true, the segment on the left is valid and you can then read the scan's output index, also one step on the left, to get the segment's starting index. Otherwise, it's invalid and you can skip it.

For the above example, we have three `1`s:

1. Index 1. Index 0's flag is `0`, so skip.
2. Index 3. Index 2's flag is `1`, so skip again.
3. Index 6. Index 5's flag is `2` (active), so read the scan output index: 3. Segment bounds are $[3,6]$.

# CUDA *really* favours this

If you've read any of my other posts, you'll know I really like CUB. CUB's scans - like `DeviceScan::InclusiveScan` - are all *really* efficient, primarily because they do the scan via a tree. The result is that this is usually faster than anything I (you) will ever write, so we should just use it.

However, in my opinion, there's another more compelling reason to use a scan in this scenario: *simplicity*. Consider the original naive method; find a `2`, find surrounding `1`s, rinse and repeat.

A blind conversion of this into CUDA would probably look something like:

1. Launch kernel to cover all pixels; simple grid-stride.
2. At every pixel, check if it's a `2`.
3. If `2`, search outwards for the `1`s.

To do this search in step 3 from the kernel, you have a few options, all of which are terrible for occupancy and/or simplicity:

- Use the thread that found the `2`. If you think this is acceptable Jensen Huang will personally strangle you with his leather jacket.
- Use the warp/block. Still unacceptable. Also complicates things because you will have to somehow gather the candidates and then iterate on them cooperatively.
- Use a new kernel. Or use CDP from the thread that found the `2` (I will silently judge you for this).

Maybe I'm being subjective here, but all of these pale in simplicity and efficiency to the scan provided by CUB; do it once, *and* use all of the GPU's resources at all (kinda) times. We trade time spent implementing for time spent thinking, and get performance in the process.

# One extra requirement

Not all is fine and dandy with CUDA's scans though. They impose one further constraint: **associativity**.

For those who have forgotten your math, this implies the following:

$$
f(f(a,b), c) = f(a, f(b,c))
$$

This is elegant to look at, but easy to stumble over. In the 1-2-1 scan I just described, it is possible to write a binary functor that *does not* obey this. By that I actually mean that the functor written above is a perfect example of one that violates associativity.

To see why, consider the following example, with the transform applied before the scan:

```
 In 0 1 2
Sdx - 1 -
```

We can define these 3 scan-pairs - (flag, index) - mathematically as 

$$
a = (0, -1)
$$

$$
b = (1, 1)
$$

$$
c = (2, -1)
$$

We can test exactly what the previous functor did on these 3 points pair-wise

$$
f(a, b) = (1, 1)
$$

$$
f(b,c) = (2,1)
$$

and then test the associativity

$$
f(f(a,b), c) = f((1,1), (2,-1)) = (2,1)
$$

$$
f(a,f(b,c)) = f((0,-1),(2,1)) = (2,-1)
$$

Thus, in this case, the two are not equal, proving that the binary functor as defined above is actually *not associative*. This means that it is not guaranteed to produce the correct (intended) results when using `cub::DeviceScan`.

But what actually happened here? The fact is that using the flag to *decide* between the `if/else` paths for the functor in this case breaks associativity, because it gets mutated in one of the paths.

In the **extend** path, we perform the `max` operation and keep the larger value. In this case, that changes a `1` flag to a `2`, and that switches the path from the `if`(reset) to the `else`(extend).

The solution is hence to ensure that this does not happen, by using some other method to decide between the reset/extend paths:

```cpp
FlagAndIndex operator()(
	FlagAndIndex a,
	FlagAndIndex b
){
	// our transform changed all non-reset indices to 
	// negative values, so we can use the index as
	// the decider instead
	if (b.idx >= 0) // reset
		return b; // we 'forget' everything before
	else // extend
		// copy ('drag') the earlier index,
		// take the larger flag (so we keep 'activations')
		return {max(a.flag, b.flag), a.idx};
}
```

With this, associativity is restored! ~~And you won't get a nasty surprise when some scans suddenly don't work the way you expected.~~
