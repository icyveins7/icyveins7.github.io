---
title: "Some notes on 2D real-to-complex Fourier transforms"
date: 2024-07-15T20:00:00+08:00
draft: false
toc: false
images:
tags:
  - "dft"
  - "c++"
  - "r2c"
  - "ipp"
  - "cufft"
  - "numpy"
  - "python"
---

> IPP in particular has some very niche ways of packing R2C DFT output, but otherwise there's a few pointers here to keep in mind for how they are implemented in most libraries.

It's pretty well known that the output of a Fourier transform of real inputs has symmetric properties. This is due to the fact that real waves consist of two conjugate pairs of complex exponentials.

What may be a bit less obvious (at least to me, when examining some programming libraries) is exactly how many useful output elements there are, and under which scenarios. In particular, I looked at IPP (which has some special packed structure), and cuFFT/NumPy/SciPy (which follow the FFTW structure I think).

# The 1D Definition

We should start here before going on to 2D things. It's easily wiki-able, but here's our 1D DFT:

$$
X_k = \sum_n x_n e^{-i 2 \pi \frac{kn}{N}}, \text{for } n \in [0,…,N-1]
$$

where $x_n$ consists of real elements in a length $N$ array.

## Conjugate pairs

The first obvious redundancy happens when you recognise

$$
\begin{align}
X_{N-k} &= \sum_n x_n e^{-i 2 \pi \frac{(N-k)n}{N}} \\
&= \sum_n x_n e^{-i 2 \pi \frac{Nn}{N}} e^{i 2 \pi \frac{kn}{N}} \\
&= \sum_n x_n e^{i 2 \pi \frac{kn}{N}} \\
&= {X_k}^*
\end{align}
$$
So *roughly* half the output is redundant. But for programming purposes, let’s be a bit more specific.

## Odd $N$

The 0-th element is self-conjugate, so it must be completely real. The remaining elements can be paired up, so in general there will be $\frac{N-1}{2}$ conjugate pairs of complex values that are useful.

In total this makes for $2\frac{N-1}{2} + 1 = N$ useful real values.
## Even $N$

The 0-th element is self-conjugate and is completely real again. However there is another element at $X_{N/2} = {X_{N-N/2}}^*$ that by definition must also be self-conjugate, and hence completely real. The rest of the elements then comprise the $\frac{N-2}{2}$ conjugate pairs of useful complex values.

This again makes for a total of $\frac{N-2}{2} + 2 = N$ useful real values.

> In all cases, the number of useful, real values is equal to $N$. This shouldn’t be too surprising since the dimensionality cannot change!

# Library output formats
## FFTW-like complex output (NumPy/SciPy/cuFFT/IPP CCS)

Most libraries return a fully complex-valued, truncated output, with length $N/2 +1$. This handles both odd and even valued $N$ correctly (assuming you do integer division for $N/2$), with the last value having a non-zero imaginary component only for odd $N$.

This is equivalent to converting the input to complex first - NumPy/SciPy will do this for you - running a standard FFT, then dropping the second half(ish).

```python
# x has length 5
In [8]: sp.fft.fft(x)
Out[8]: array([-2.3526919 -0.j        ,  1.37635917-0.27259233j,  0.5953175 -0.55606247j,  0.5953175 +0.55606247j,  1.37635917+0.27259233j])

# So this stops at 5//2 + 1 = 3, and last element has a non-zero imaginary component
In [9]: sp.fft.rfft(x)
Out[9]: array([-2.3526919 +0.j        ,  1.37635917-0.27259233j,  0.5953175 -0.55606247j])

# .....

# Now x has length 6
In [11]: sp.fft.fft(x)
Out[11]: array([-0.391538  -0.j        , -2.01281022-0.2942553j ,  2.76346742+2.30952267j,  1.36395615-0.j        ,  2.76346742-2.30952267j, -2.01281022+0.2942553j ])

# And this stops at 6//2 + 1 = 4, with the last element having zero for its imaginary component
In [12]: sp.fft.rfft(x)
Out[12]: array([-0.391538  +0.j        , -2.01281022-0.2942553j ,  2.76346742+2.30952267j,  1.36395615+0.j        ])
```

So far this shouldn't be hard to accept; you're just taking all the positive frequencies from a standard Fourier transform (with some handling for the edge bin).


## Packed output (IPP Pack)

IPP has packed formats documented for its [signal](https://www.intel.com/content/www/us/en/docs/ipp/developer-guide-reference/2021-12/packed-formats.html) processing library part. I think the documentation there describes it sufficiently well, but in case you'd like to see a concrete example, here's the extension using the above python array:

```python
# This is the output of the rfft for the length 6 array
In [14]: y
Out[14]: array([-0.391538  +0.j        , -2.01281022-0.2942553j ,  2.76346742+2.30952267j,  1.36395615+0.j        ])

# And this is how IPP would effectively pack the output
In [15]: np.hstack((y[0].real, y[1:-1].view(np.float64), y[-1].real))
Out[15]: array([-0.391538  , -2.01281022, -0.2942553 ,  2.76346742,  2.30952267,  1.36395615])
```

Since there are always $N$ real values in the output, we can drop the 0-valued imaginary components (the 0-index, and the N/2-index for the even $N$ case) and simply squeeze everything together. This would make the output a bit shorter.

Notably, I haven’t seen any other library opt to this, since it would require operating on the output in a non-uniform way; you would have to remember that the first (and possibly the last) output has 1 real element, while everything in between has 2 real elements (1 complex element). That means you can’t traverse it with the same pointer.

Understandably, this is probably why it isn’t seen elsewhere. IPP has a variation of choices in what output formats you can select - but not always, since in its image processing section you are *forced* to use this output format, and then convert it to another format if you wish after.

# 2D transforms and their R2C output dimensions

At first glance, you might have thought that since 1D transforms can be defined by half the output, 2D transforms could be defined by $1/4$ of them right?

Okay if not, then you’re smarter than I was. After I had realised the dimensionality argument for the 1D case, the 2D case became equally clear.

> In an $M \times N$ real matrix, the output of the transform must contain $MN$ real, useful values.

It shouldn’t be hard to see that this means that you can still only drop about half of the full complex 2D DFT output.

But how do libraries choose what to keep? It turns out everyone seems to have *chosen to agree* on this part.

## Slice the fastest changing dimension

The title says it all. In a typical array defined by, for example

```
std::complex<float> out[a][b];
```

the `b` index is the one that gets truncated. This also follows for 3D transforms, though I’m not going to bother with that here.

Again, there’s no mathematical reason behind this; it’s merely a convention that seems to be held by every library: NumPy/SciPy, cuFFT, etc.

To see why you could slice it the other way, let’s consider an example.

```python
# We now have a random 6x6 input array
In [25]: sp.fft.fft2(x)
Out[25]:
array([[ 11.4684-0.0000e+00j,   7.1864+9.1749e+00j,   4.1346-3.6415e+00j,   0.9137-0.0000e+00j,   4.1346+3.6415e+00j,   7.1864-9.1749e+00j],
       [  5.6947+1.9658e-01j,  -0.5213+2.2622e+00j, -10.9693-4.7503e+00j,  -3.6554+2.2839e-01j,  -1.3505+4.1515e+00j,   3.4155+3.9835e+00j],
       [  8.1551+2.6689e+00j,  -5.7527-4.1722e+00j,   0.6473-1.1867e+00j,  -4.3752+5.1961e+00j,  -3.7517+1.5745e+00j,  -2.1656+9.6490e+00j],
       [ -1.089 -2.9802e-08j,   1.31  -1.0136e+01j,  -4.7408-7.9127e-01j,   8.455 +3.7253e-08j,  -4.7408+7.9127e-01j,   1.31  +1.0136e+01j],
       [  8.1551-2.6689e+00j,  -2.1656-9.6490e+00j,  -3.7517-1.5745e+00j,  -4.3752-5.1961e+00j,   0.6473+1.1867e+00j,  -5.7527+4.1722e+00j],
       [  5.6947-1.9658e-01j,   3.4155-3.9835e+00j,  -1.3505-4.1515e+00j,  -3.6554-2.2839e-01j, -10.9693+4.7503e+00j,  -0.5213-2.2622e+00j]], dtype=complex64)

# As you can see the output of this is simply indexing the output above like [:,:4]
# i.e. taking all rows, but cutting at the equivalent 1D required column
# Note that numpy arrays are row-major by default, so this has truncated the fastest changing dimension
In [26]: sp.fft.rfft2(x)
Out[26]:
array([[ 11.4684+0.0000e+00j,   7.1864+9.1749e+00j,   4.1346-3.6415e+00j,   0.9137+0.0000e+00j],
       [  5.6947+1.9658e-01j,  -0.5213+2.2622e+00j, -10.9693-4.7503e+00j,  -3.6554+2.2839e-01j],
       [  8.1551+2.6689e+00j,  -5.7527-4.1722e+00j,   0.6473-1.1867e+00j,  -4.3752+5.1961e+00j],
       [ -1.089 +2.9802e-08j,   1.31  -1.0136e+01j,  -4.7408-7.9127e-01j,   8.455 -3.7253e-08j],
       [  8.1551-2.6689e+00j,  -2.1656-9.6490e+00j,  -3.7517-1.5745e+00j,  -4.3752-5.1961e+00j],
       [  5.6947-1.9658e-01j,   3.4155-3.9835e+00j,  -1.3505-4.1515e+00j,  -3.6554-2.2839e-01j]], dtype=complex64)
```

If you look closely at the original, *full* output of the 2D FFT, you can separate it into a few block matrix sections that contain the conjugate pairs:

```
|A|B|B|B|B|B|
|C|D|D|D|D|D|
|C|D|D|D|D|D|
|C|D|D|D|D|D|
|C|D|D|D|D|D|
|C|D|D|D|D|D|
```

In each block, the conjugate pairs are nicely positioned on what I'd like to call _opposite ends of a line passing through the block origin_. Here are some conjugate pairs you can verify, together with the example output above:

```
# Block D
|A|B|B|B|B|B|
|C|x| | | | |
|C| | | | | |
|C| | | | | |
|C| | | | | |
|C| | | | |x|

# Block D
|A|B|B|B|B|B|
|C| |x| | | |
|C| | | | | |
|C| | | | | |
|C| | | | | |
|C| | | |x| |

# Block D
|A|B|B|B|B|B|
|C| | | | | |
|C| | | |x| |
|C| | | | | |
|C| |x| | | |
|C| | | | | |

# Block D, self-conjugate i.e. 0 imaginary component
# Note that numerical accuracy sometimes leaves it as a small number instead..
|A|B|B|B|B|B|
|C| | | | | |
|C| | | | | |
|C| | |x| | |
|C| | | | | |
|C| | | | | |

# Block C
|A|B|B|B|B|B|
| |D|D|D|D|D|
|x|D|D|D|D|D|
| |D|D|D|D|D|
|x|D|D|D|D|D|
| |D|D|D|D|D|

# Block C self-conjugate
|A|B|B|B|B|B|
| |D|D|D|D|D|
| |D|D|D|D|D|
|x|D|D|D|D|D|
| |D|D|D|D|D|
| |D|D|D|D|D|
```

Of course, this is just graphically representing the conjugate pair relationship in 2D:

$$
X_{M-k, N-l} = {X_{k,l}}^*
$$

The idea is you'd need to keep at least one of each conjugate pair to have non-redundant output; and importantly, since the conjugate pairs are in block matrices, you can target to keep half of each block. The way all the libraries do it (probably because they do the transforms down the rows first, then down the columns) is to keep half of block $B$ and all of block $C$, chopping block $D$ vertically in the process.

```
|A|B|B|B|-|-|
|C|D|D|D|-|-|
|C|D|D|D|-|-|
|C|D|D|D|-|-|
|C|D|D|D|-|-|
|C|D|D|D|-|-|
```

However you could easily just chop block $D$ horizontally instead, and still keep an equivalent set of non-redundant output:

```
|A|B|B|B|B|B|
|C|D|D|D|D|D|
|C|D|D|D|D|D|
|C|D|D|D|D|D|
|-|-|-|-|-|-|
|-|-|-|-|-|-|
```

## And finally, the IPP 2D packed format..

The [RCPack2D](https://www.intel.com/content/www/us/en/docs/ipp/developer-guide-reference/2021-12/real-complex-packed-rcpack2d-format.html) format is found only in the image processing section.

Now this is really something. In theory, this is just extending the 1D packed format idea to 2D; everytime you meet an imaginary component of 0, you skip it (hence the __Packed__), but there's a little more nuance to it.

The first thing to note is that the previous method can still contain redundant values after slicing. In our $6 \times 6$ example above, the final truncated output still has the following redundant pairs:


```
|A|B|B|B|-|-|
|x|D|D|D|-|-|
| |D|D|D|-|-|
| |D|D|D|-|-|
| |D|D|D|-|-|
|x|D|D|D|-|-|

|A|B|B|B|-|-|
| |D|D|D|-|-|
|x|D|D|D|-|-|
| |D|D|D|-|-|
|x|D|D|D|-|-|
| |D|D|D|-|-|

|A|B|B|B|-|-|
|C| | |x|-|-|
|C| | | |-|-|
|C| | | |-|-|
|C| | | |-|-|
|C| | |x|-|-|

|A|B|B|B|-|-|
|C| | | |-|-|
|C| | |x|-|-|
|C| | | |-|-|
|C| | |x|-|-|
|C| | | |-|-|
```

I didn't include the self-conjugate centre indices in blocks $B,C,D$ as they should be treated the same as the 1D case.

So there can be a _shorter_ first and last column. Hopefully, it shouldn't be too hard to see that for an odd number of columns, the last column will not have this issue.

Okay, so let's get rid of the extra redundant indices,

```
|A|B|B|B|-|-|
|C|D|D|D|-|-|
|C|D|D|D|-|-|
|C|D|D|D|-|-|
|-|D|D|-|-|-|
|-|D|D|-|-|-|
```

and then let's label the fully real \(R\) and fully complex (*) indices:

```
|R|*|*|R|-|-|
|*|*|*|*|-|-|
|*|*|*|*|-|-|
|R|*|*|R|-|-|
|-|*|*|-|-|-|
|-|*|*|-|-|-|
```

Here's where IPP's packed format comes in:

1. The first (and in this case also the last) column is flattened to real values _into a column_. This becomes a length-6 column - 2 from reals and $4=2 \times 2$ from complex values - and is written into the first (and last) column of a $6 \times 6$ matrix.
2. The columns in between, which are flattened to real values _into their respective rows_. This will become - in this case - 6 rows and 4 columns of real values.

Hence, the final output becomes a $6 \times 6$ real-valued matrix, with no redundant values whatsoever.

## Sounds like a lot of work for not a lot of gain?

I think so too. By the logic above, you'd save roughly $2M/2 = M$ values worth of memory for a general $M \times N$ matrix. Even worse, if someone had to operate on this other than IPP, you'd have to unfold this anyway, or have to remember to deal with these first/last column shenanigans.

