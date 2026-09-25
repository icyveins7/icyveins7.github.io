---
title: "Interesting Tidbits from GTC 2025: CUDA Graphs"
date: 2025-03-25T20:00:00+08:00
draft: false
toc: false
images:
tags:
  - "cuda"
  - "gtc"
---

> In the 2nd post of this series, I give a short introduction on something that is also not particularly new, but new to me - CUDA Graphs!

As stated in my first post of this series, this topic isn't particularly new per-se. Indeed, it looks like it has been out since 2019. But maybe CUDA graphs have increased in relevance now that GPUs are more powerful. As usual, a good starting reference is NVIDIA's own [blogpost](https://developer.nvidia.com/blog/cuda-graphs/).

Note that CUDA graphs require a minimum toolkit version of 12.4, with some additional features being present in the newest (as of this writing) version, 12.8.

# Why Use Graphs?

Perfectly reasonable question. The problem we are trying to combat when using graphs is _overhead_. This is really only present when our kernels are _short_, _repeated_ or both.

In those scenarios, the overhead of the kernel launch becomes __comparable__ to the duration of the kernel itself. This is often observed in an Nsight Systems timeline where there are visible gaps between the kernels after zooming in (yes, of course there are always gaps, but its the relative size of the gap to the size of the kernel that matters).

# How It Works in a Nutshell

I liken this process to the distinction between _compiled_ and _interpreted_ languages. Interpreted languages are usually slower - yes, I know many nowadays have JIT and can be optimized pretty well, but bear with me for the comparison - because each command or instruction has to be submitted individually. This is necessary since all code is parsed only at runtime, so to balance start-up time with actual run duration, some assumptions are made and/or not all optimizations are performed. _Of course, this is a vast simplification, but the idea still stands._

Compiled languages don't suffer from this, since you as the programmer get to choose what occurs at runtime and what occurs at compile time. This affords the compiler a lot of flexibility to optimize as much as possible.

In the same vein, CUDA graphs offer the compiler a way to 'compile' multiple kernels together, and also submitting the work in a more 'compressed, single call'. This is, in fact, how it appears in a standard Nsight Systems profiler timeline if no additional settings are specified.

In plain English, CUDA graphs allows you to tell the device to

> do these N kernels in a row

rather than

> do this kernel, then do this kernel, then do this kernel ....

# Method 1: Record Your Kernels

This is the simpler method to convert existing code to the graph format. As seen from the blog post, you can do the following:

```cpp
bool graphCreated=false;
cudaGraph_t graph;
cudaGraphExec_t instance;
for(int istep=0; istep<NSTEP; istep++){
  // Create graph only on first iteration
  if(!graphCreated){
	// Start 'recording'
    cudaStreamBeginCapture(stream, cudaStreamCaptureModeGlobal);
    for(int ikrnl=0; ikrnl<NKERNEL; ikrnl++){
      shortKernel<<<blocks, threads, 0, stream>>>(out_d, in_d);
    }
    // Stop 'recording'
    cudaStreamEndCapture(stream, &graph);
    cudaGraphInstantiate(&instance, graph, NULL, NULL, 0);
    graphCreated=true;
  }
  // Launch graph on every iteration
  cudaGraphLaunch(instance, stream);
  cudaStreamSynchronize(stream);
}
```

In fact, I think you can probably just do a warm-up recording before any of the actual iterations, as long as the data pointers and all other parameters are valid. This is, of course, to avoid the much more expensive graph instantiation in your actual hot loops (even if it's only the first iteration).

# Method 2: Explicit API Calls

This method requires a definition of a graph in the more traditional sense: creating and connecting nodes to define dependencies.

As seen in the [documentation](https://docs.nvidia.com/cuda/cuda-c-programming-guide/index.html#creating-a-graph-using-graph-apis), this looks like the following:

```cpp
/*
  A
 / \
B   C
 \ /
  D
*/

// Create the graph - it starts out empty
cudaGraphCreate(&graph, 0);

// For the purpose of this example, we'll create
// the nodes separately from the dependencies to
// demonstrate that it can be done in two stages.
// Note that dependencies can also be specified
// at node creation.
cudaGraphAddKernelNode(&a, graph, NULL, 0, &nodeParams);
cudaGraphAddKernelNode(&b, graph, NULL, 0, &nodeParams);
cudaGraphAddKernelNode(&c, graph, NULL, 0, &nodeParams);
cudaGraphAddKernelNode(&d, graph, NULL, 0, &nodeParams);
// All the parameters go into `nodeParams`, including
// kernel launch parameters, the kernel function pointer,
// kernel input arguments etc.

// Now set up dependencies on each node
cudaGraphAddDependencies(graph, &a, &b, 1);     // A->B
cudaGraphAddDependencies(graph, &a, &c, 1);     // A->C
cudaGraphAddDependencies(graph, &b, &d, 1);     // B->D
cudaGraphAddDependencies(graph, &c, &d, 1);     // C->D
```

You might be wondering, why would you want to do this? Well, using the API to create graphs opens up a solution to 1 particular issue I suspect every CUDA programmer will eventually reach..

## Conditional Nodes

There comes a time where your overarching logic must diverge based on some condition. Common examples include: iterative loop stops after convergence, check for thresholds etc.

In these cases, you might have some chain of logic which would be different depending on (effectively) some boolean. Now you have 2 options:

1. Combine all the divergent logic into the kernel itself, then launch just 1 kernel, within which the divergent logic is executed at a thread level. This can get very unwieldy very quickly, and may not even be possible.
2. Copy the boolean out to the host (ugly, and also likely to be slow). Read the boolean in the host, and then decide which kernel or chain of kernels to execute.

Refer to [this blogpost](https://developer.nvidia.com/blog/dynamic-control-flow-in-cuda-graphs-with-conditional-nodes/) for an excellent introduction to this, but I'll summarise here.

Using CUDA graphs, you can now define a conditional handle via

```cpp
cudaGraphConditionalHandle handle;
cudaGraphConditionalHandleCreate(&handle, graph);
```

You would then use this inside a kernel like this:

```cpp
__global__ void setHandle(cudaGraphConditionalHandle handle)
{
	// ...
	// as long as value is non-zero it is 'true'
    cudaGraphSetConditional(handle, value);
}
```

The graph will now execute the conditional node without returning the boolean to the host, and you have the freedom to define (potentially) an entire graph of nodes in the `if true` expression.
