---
title: "When circumstances don’t allow you to use unique_ptr"
date: 2025-02-23T20:00:00+08:00
draft: false
toc: false
images:
tags:
  - "c++"
  - "memory"
  - "manager"
  - "unique_ptr"
---

> Some background on my single header file memory manager and why I made it..

Picture this. You are working on a codebase with some ~~lazy~~ ~~inept~~ questionable decisions from external forces. You are given a class to work on. For the purposes of this post let’s call it `MyClass`.

The class will have its methods invoked in the following order:

1. `MyClass()` i.e. the constructor.
2. `setup(…)`
3. `run(…)`
4. `teardown()`
5.  Repeat 2-4 many times.
6.  `~MyClass()` i.e. the destructor.

This class will be reused in multiple different scenarios repeatedly, with different input data.

Here’s the problem: you also need to allocate different things/sizes of arrays depending on the data. Hence, there is effectively no way to pre-allocate this in the constructor. You will have to do this in `setup()`, and then destroy or free these in `teardown()`.

# (Over-)dramatisation

Them: You must pre-allocate your data in `setup` and remember to free it in `teardown`. 

Me: Sure, but I’d like to use a `unique_ptr` or similar smart pointers to do it so that my colleagues and I don’t have to spend extra time searching for memory leak bugs. I can’t do this if we don’t call the destructor for automatic cleanups.

Them: We cannot keep destroying and reinstantiating the objects. It will be too much overhead.

Me: But we are allocating *gigabytes* of data anyway, the constructor overhead is going to be negligible in the midst of all this.

Them: …

Me: ??? (Your brain is the overhead)

# A compromise

With no other solution, I decided to write my own. Well, I had an idea of a memory manager, and then technically I prompted ChatGPT to write one for me. 

Specifically, I needed to make sure it would handle the following:

1. Ability to handle arbitrary POD type heap allocations. Stuff like `new arr[size]` would need to be replaceable.
2. Ability to handle custom class constructors. Stuff like `new SomeClass(arg1, arg2)` would need to be replaceable.

A few prompts, and some manual edits later, [I had what I wanted](https://github.com/icyveins7/memory-manager).

The premise is pretty simple: have a memory block structure and hide your allocations inside them.

Hold them all in an `std::vector`, and then `.clear()` it when finished with them.

All the references disappear when the vector is cleared, and each memory block will call its own functor to free the internal memory appropriately i.e. `delete` or `delete[]`.

# Why not just hold a vector of `MemoryBlock`s?

Great question. ChatGPT reminded me; I’ll paraphrase it here with the following important question:

> What happens when the vector is resized?

If we had taken the code I linked above and simply changed `vector<unique_ptr<MemoryBlock>>` to `vector<MemoryBlock>`, the resize would have done the following:

1. Copied the MemoryBlocks over. This is just a copy of each pointer we allocated. This is fine.
2. Freed the existing memory holding the original memory blocks. This would call each memory block’s destructor, which would call each functor, which would free the internally kept pointer. *This is not fine.*
3. The pointers we copied over now point to nothing.
4. Freeing the new resized vector would result in double frees (or another resize would also do this).

The typical solution is to redesign the `MemoryBlock` so that the copy constructors, assignment operators etc are well-defined. The rule of 5 strikes again.

In this case, we really just want to make sure that the internal pointer (which implies the `MemoryBlock` itself) is move-only, so that when the vector resizes it will not delete the internal pointers at any point. But this is exactly what using `unique_ptr` achieves! Hence, no need to reinvent the wheel.

# Jokes on me..

In the end, they still didn’t want to implement this. ‘Unnecessary code changes’..
