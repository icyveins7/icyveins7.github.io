---
title: "Some heuristic proofs for cyclostationary methods"
date: 2024-04-16T20:00:00+08:00
draft: false
toc: false
images:
tags:
  - "dsp"
  - "cyclostationary"
  - "proof"
---

> Carrier offset and baud rate estimation can be done blindly using cyclostationary (cyclic moment) methods, but why?

# Introduction

In [ReImage](https://github.com/icyveins7/reimage), there are options to blindly estimate a signal's baud rate or residual carrier offset. These require some working knowledge, and only work on some types of modulations like PSK. But why do they work, and how do we explain the peaks we see in the resulting spectra?

First, some terminology. Here I'll often mention the exponents in a _CM_ XY form: this refers to applying an exponent onto a signal with a total of $X$, where $Y$ of that is the conjugate. These refer to the cyclic moments.

For example, _CM_ 20 refers to $x^2$ whereas _CM_ 21 refers to $x x^*$.

To see a proper treatment, refer to [the man himself](https://cyclostationary.blog/2019/04/26/simple-synchronization-using-csp/). Otherwise, onwards we go..


# Carrier offset estimation

Let's consider a generic QPSK signal given by:

$$
s(t) = \left[ \sum_k m_k (\delta(t - kT) \star h(t)) \right] e^{i 2 \pi f_c t}
$$

where the symbols are given by

$$
m_k \in \{1, i, -1, -i\}
$$

The baud period here is specified by $T$ and the carrier offset to be estimated is given by $f_c$.

The correct thing to do here is to find some values of exponents where the result contains _additive tones_. These will show up in a specturm of FFT plot as peaks that can be easily measured.

We claim now the required operation is _CM_ 40 i.e.

$$
s(t) \rightarrow s^4 (t)
$$

Let's see what this does to the signal. First, observe that the exponent applied directly to the message symbols always results in 1:

$$
{m_k}^4 = 1 \, \text{for all } m_k
$$

What happens when we first try to square the terms in the square brackets?

There are 2 terms at play here: intra-term products and inter-term products.

Let's first consider the inter-term products:

$$
m_k m_{k'} (\delta(t - kT) \star h(t)) (\delta(t - k'T) \star h(t))
$$

It shouldn't be too difficult to see that this should tend to 0, since for a random variable $m_k$, the expectation value of $m_k m_{k'}$ should approach 0. Moreover, most reasonable time-limited pulse shapes $h(t)$ only have non-negligible support within 1 symbol i.e. within $T$ of each other. Hence, the contribution of non-zero products is also limited.

Hence we turn our attention to the intra-term products:

$$
{m_k}^2 (\delta(t-kT) \star h(t))^2
$$

We square again to get our exponent of 4, obtaining

$$
{m_k}^4 (\delta(t-kT) \star h(t))^4 = (\delta(t-kT) \star h(t))^4
$$

where we have substituted ${m_k}^4 = 1$.

What can we say about this final expression? Let's bring the summation back into the picture:

$$
\sum_k (\delta(t-kT) \star h(t))^4
$$

We start the analysis by considering what would happen if we used a $\delta$ function as a pulse:

$$
\sum_k (\delta(t-kT) \star h(t))^4 \rightarrow \sum_k (\delta(t-kT))^4 = \sum_k \delta(t-kT)
$$

Well the Fourier transform of this Dirac comb is simply another Dirac comb ([see this](https://dspillustrations.com/pages/posts/misc/the-dirac-comb-and-its-fourier-transform.html)):

$$
\mathcal{F}\left[\sum_k \delta(t-kT)\right] = \sum_k \delta\left(f - \frac{k}{T}\right)
$$

Let's put this together with the other parts of $s(t)$ to get

$$
\begin{align}
\mathcal{F}\left[s(t)^4\right] &\approx \sum_k \delta\left(f - \frac{k}{T}\right) \star \delta(f - 4f_c)\\
&= \sum_k \delta(f - 4f_c - \frac{k}{T} )
\end{align}
$$

where we have left out any inter-term products and also the Fourier transform of ${m_k}^4$.

What does this suggest to us? The spectrum consists of components centred around $f = 4f_c$, with steps of $\pm1/T$, otherwise known as the baud rate.

But here we assumed the infinitesimal pulse shape $\delta$; what if we revert to a more practical pulse shape like an RRC? Heuristically, we need only consider the properties of Fourier transforms: the more support it has in time, the less support its transform has in frequency. Hence, we expect to have the Dirac comb in frequency space be multiplied (since we convolved with $h(t)$) with a pulse of some _bandwidth_.

$$
\begin{align}
\sum_k (\delta(t-kT) \star h(t))^4 &= \sum_k h^4(t-kT) \\
&= \sum_k \delta(t - kT) \star h^4(t)
\end{align}
$$

$$
\begin{align}
\mathcal{F}\left[ \sum_k \delta(t-kT) \star h^4(t) \right] &= \sum_k  \delta(f - \frac{k}{T}) \times \mathcal{F} \left[ h^4 (t) \right] \\
&= \sum_k  \delta(f - \frac{k}{T}) \times H(f)
\end{align}
$$

That is, the amplitude of the Dirac comb in frequency space has an _envelope_ defined by the Fourier transform of $h^4 (t) = H(f)$. Together with the other $\delta$-function, this shifts the entire enveloped Dirac comb to centre around $4f_c$.

Hence, we expect to see a large peak in the spectrum at $4f_c$, with smaller peaks uniformly at $1/T$ spacing.




