---
title: "Old dog, old C struct tricks"
date: 2025-07-14T20:00:00+08:00
draft: false
toc: false
images:
tags:
  - "c"
  - "struct"
  - "memory"
---

> Look at this code snippet and tell me with a straight face you didn't think it was a memory leak at first either..

A while ago I worked on some code which contained a container that looked like this:

```cpp
struct S
{
  size_t sz;
  int data[1];

  void Set(size_t _sz, int *_data){
    sz = _sz;
    memcpy(data, _data, sizeof(int)*sz);
  }
}
```

If you're a modern programmer like me, you might be looking at this and thinking: *there's no way this isn't a flagrant memory violation in almost every case*. Right? Well, it turns out this is something them old boys before C99 used to do.

# Cool kids are manual and contiguous

The above `struct` works because it is meant to be manually managed, and because the array is *the last field of the struct*. 

The idea was that you would heap-allocate the struct with *padded bytes* according to your required array length; this was the poor man's dynamic array allocation back then.

```cpp
struct S* sptr = (struct S*)malloc(sizeof(struct S) + sizeof(int) * (N-1));
// we have -1 because the struct already has room for 1 int
```

Now, when you access `sptr.data[i]`, you are simply looking farther into the memory block you have already allocated. You could easily range-protect this access with an `std::vector`-like method; the cool thing here is that we get to have **all the memory in one contiguous block**.

This is unlike `std::vector` (and other containers) where the parameters like the size and capacity live on the stack, whereas the data is heap-allocated somewhere else.

Nowadays, post-C99, the syntax is a bit less misleading:

```cpp
struct S
{
  size_t sz;
  int data[]; // this now indicates dynamic length. Still needs to be the last member though, and still needs manual management

  void Set(size_t _sz, int *_data){
    sz = _sz;
    memcpy(data, _data, sizeof(int)*sz);
  }
}
```

The benefits are still the same though! Parameters and data, living happily next to each other!
# Premature optimization is the..

Yeah yeah, but I looked into it anyway. A quick chat with our favourite GPT brought up a topic I expected: cache-lines. But it was somewhat opposite to my intuition, which was interesting.

I thought that having things contiguous would benefit small data the most, but it turns out that the cache changes things (*surprise surprise*).

The crucial idea here is the **total size of all the structs**. Assuming you have to iterate/read all of them at least once, there shouldn't be any significant difference between the small struct and the large struct if all of them fit in L1, because you pull data in cache lines anyway.

The real driver of costs comes from reaching further out to L2, L3 and RAM. This is where contiguity - and the associated prefetching - plays a huge role. Having them all read one after another will assist hardware prefetching, which is present in most CPUs these days.

I will be the first to admit that I didn't test or benchmark this myself, because this is out of my concern. I merely wanted to highlight this interesting old C trick, and hope someone else recognizes that sometimes, boomers' code isn't the trash you think it is.
