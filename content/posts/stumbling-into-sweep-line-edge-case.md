---
title: "Stumbling into a sweep line algorithm's edge case"
date: 2025-09-10T20:00:00+08:00
draft: false
toc: false
images:
tags:
  - "c++"
  - "sweepline"
  - "algorithm"
---

> Mistakes that you should learn from, lesson 1..

I recently wrote some code for interval merging. Every leetcoder can probably recite 15 algorithms to do this in their sleep, but I figured it out myself in this case, and in the process fell into an edge case.

Here's to writing down your mistakes.

# Interval merging

The generic interval merging problem provides you with a list of start/stop pairs which denote individual intervals. Each interval is then to be merged with any other overlapping intervals; an overlap is defined by at least 1 element being shared between 2 intervals.

There are several variants of this problem, depending on how the data structure of the input pairs, but I'll focus on the one I ended up with: 

```cpp
struct Event {
  size_t idx;
  uint8_t flag; // 1: start, 0: stop
}
```

# Always be sorting

The first step in all interval merge solutions is to sort the intervals. This is, of course, to put adjacent and possibly overlapping intervals next to each other so they can be compared over a single pass.

In my case, the disparate `Event`s need to be sorted, so this is as simple as using `std::sort` with a defined comparison operator:

```cpp
bool operator<(const Event& rhs) {
  return idx < rhs.idx;
}
```

# *Energy* states determine closure

The sweep line algorithm now essentially *sweeps* through the sorted list of on (1) and off (0) events. At each event, it does the following:

1. If the current counter is 0 and the event flag is 1, add a new open interval with an unknown ending.
2. If the current counter is more than 0 and the event flag is 1, simply increment the counter.
3. If the event flag is 0, decrement the counter.
4. If the counter reaches 0, close the current interval.
```
# Evolution of counter over events
X-----X  X-----X
|   X-------X  |
|   | |  |  |  |
1   2 1  2  1  0
```

The physicist in me likes to see this as going up and down energy states (*no? just me?*).

# The mistake (and the fix)

There is one special case that causes this to fall apart, and it has to do with the way we defined (or failed to properly define) the sorting order.

Because we deal only with events, and the above comparison operator only checks the index, there is no guarantee of whether an **on** event will come before an **off** event if both of them have the same index, *and this is specifically what we require*.

To see why, simply consider the below scenario, which should result in a single merged interval.

```
X----X
     X-----X
```

We necessarily have the following events:

- Index $a$, flag 1
- Index $b$, flag 0
- Index $b$, flag 1
- Index $c$, flag 0

Ideally, after sorting, due to the counter mechanism outlined above, we would need the two events with the flag 1 to be next to each other. This would result in the counter changing as $1 \rightarrow 2 \rightarrow 1 \rightarrow 0$ as intended, merging the two intervals.

However, since we did not check for this, the above order may remain unchanged, causing the counter to do $1 \rightarrow 0$ twice, leaving the two intervals separated, with an illogical start/stop on the same index.

~~In fact, this may result in very odd size-less intervals when there more overlaps occur. Consider the following 4 intervals:~~

```
X-----X
  X---X
      X------X
      X----X
```

~~If we just focus on the 4-way overlap in the centre, we have two 1s and two 0s as possible event flags. In the worst case, this may be ordered as $0,1,0,1$, which would result in an interval that starts and stops on the same index (from the centre 1 and 0).~~

> Edit: I previously thought the above would result in a 0 size interval, but on second thought I was wrong.

The fix is then, of course, to use flags to sort when indices match:

```cpp
bool operator<(const Event& rhs) {
  if (idx == rhs.idx)
    return idx.flag > rhs.flag; // 1s come before 0s
  else
    return idx < rhs.idx;
}
```

