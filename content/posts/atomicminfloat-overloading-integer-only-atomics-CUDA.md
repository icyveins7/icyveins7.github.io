---
title: "AtomicMinFloat; overloading integer-only atomics for floating-point numbers in CUDA"
date: 2025-03-16T20:00:00+08:00
draft: false
toc: false
images:
tags:
  - "c++"
  - "atomic"
  - "cuda"
---

> Convincing you (and myself) that with some minor edits, we can still use atomicMin for floats in CUDA..

CUDA has a set of atomic functions for _safely_ updating the same memory address with different threads. There's a whole list of them in the programming guide [here](https://docs.nvidia.com/cuda/cuda-c-programming-guide/#atomic-functions).

However, for `atomicMin` (and also `atomicMax`), there isn't an overload that works with `float`s (or `double`s , but who uses those in CUDA anyway..). For this discussion we'll just focus on `atomicMin`, but all the points we discuss can be inverted to explain `atomicMax`.

```cpp
int atomicMin(int* address, int val);
unsigned int atomicMin(unsigned int* address,
                       unsigned int val);
unsigned long long int atomicMin(unsigned long long int* address,
                                 unsigned long long int val);
long long int atomicMin(long long int* address,
                                long long int val);
```

As you can see, they all deal with integers. Okay, so how would you deal with floats? You could do some of the following:

1. Rework the code to not use atomics. This probably involves an additional kernel to do comparisons and/or some additional scratch memory. I used to do this until cooperative groups came out and the compiler started optimising warp-aggregated atomics very well.
2. Use `atomicCAS` to define the atomic operation with some funky `do-while` loop. I found this to be very inelegant.
3. Use some scaling function to turn your floating point values into an integer, preserving the order i.e. $x > y \rightarrow f(x) > f(y)$. Then just use the existing integer-based `atomicMin` calls, which would benefit from hardware acceleration.

The last method sounds tricky, but it doesn't really need to be.

# Comparing `float`s is (almost) the same as comparing `int`s

First off, I'm not the one that thought of this. See the answer [here](https://stackoverflow.com/a/72461459).  I'm going to explain why this works. To do this, we need to recap how the 32 bits in a `float`, `int` or `unsigned int` are laid out.

## IEEE-754 integers (32-bit)

This should be obvious but we'll restate it for completeness.

Unsigned integers are simply a list of bits representing the powers of 2, where the leading bit is $2^{31}$ and the ending bit is $2^0 = 1$.

Signed integers are identical to unsigned integers, but with the leading bit used for the sign bit.

## IEEE-754 single-precision floating point

Refer to this concise graphic from the [wiki page](https://en.wikipedia.org/wiki/Single-precision_floating-point_format):

![single precision bit layout](https://upload.wikimedia.org/wikipedia/commons/thumb/d/d2/Float_example.svg/1180px-Float_example.svg.png)

Here you can easily observe the following:

- First (leading, leftmost) bit is the sign bit
- Next 8 bits define the exponent $2^{E - 127}$
- Final 23 bits define the mantissa or fraction (with implicit offset), $1 + x$, with $0 \leq x < 1$

### Exponent and mantissa sections can be compared like unsigned integers

The above is the definition, but the first important property here is that both the exponent and the mantissa sections are still _ordered_, in the sense that _leading bits represent larger numbers than trailing bits_.

This is important, because then we can treat each section like its own unsigned integer space i.e. if we treat the exponent as an 8-bit unsigned integer and compare it like an 8-bit unsigned integer for two numbers with all other bits equal, then the `float` with the _'larger 8-bit unsigned integer exponent'_ is indeed the larger `float`! The same goes for the mantissa section's 23 bits.

This also holds for the combination of the 2 sections; to see why, simply consider whether the following is possible: given fraction $x$ and exponent $E$, can we write a number where $y_1 = (1+x_1) 2^{E_1} > (1+x_2) 2^{E_2} = y_2$ but one of the following occurs:

1. $x_1 \leq x_2$
2. $E_2 \leq E_1$

Again, this isn't particularly complicated when you realise you only need to consider the smallest trailing bit of the exponent. Flipping this final, rightmost exponent bit engineers a factor of 2 change to the entire number. __Since the other 23 bits represents a number $1 \leq 1 + x < 2$, it is impossible for this number to double and create a number that overcomes this factor of 2__.

Let's use the above number in the graphic as an example. The number is

$$
2^{124-127} \times (1 + 2^{-2}) = 2^{-3} \times 1.25 = 0.15625
$$
We can increment the exponent by 1 in the trailing bit to make this 

$$
2^{125-127} \times (1 + 2^{-2}) = 2^{-2} \times 1.25 = 0.3125
$$

which is double the original number, as discussed above. It should be obvious by now - since we've already established that the mantissa number has an upper bound of 2 - but let's finish up the example for completeness. What's the maximum number we can make in the fraction (turning on all the bits)? Well this is just the glorious number

$$
1 + \sum_{i=1}^{23} 2^{-i} = 1.99999988079071044921875
$$

Clearly, this number multiplied by $2^{-3}$ is still smaller by definition than 0.3125. Thus, it is possible to conclude that we can compare both mantissa and exponent sections as if it were one long 31-bit unsigned number when we are simply concerned with magnitude of numbers.

In other words, we could simply reinterpret the trailing 31 bits of a `float` as an unsigned integer and then call `atomicMin` for our purpose! But now we need to handle the leading sign bit..

# Sign bit implies a flip of min/max operations

Let's just look at a bunch of examples of floating point values after they are reinterpreted as unsigned or signed integers (shown in row-pairs). In each pair, the smaller number will be in bold.

> It is important to note that in our `atomicMin` case, the access pattern is not _symmetric_, in the sense that the `val` is usually on-chip/local, whereas `address` usually refers to some global memory which is way more expensive to access.
> 
> This means that we should consider our information to be limited to `val` alone - we _observe_ what `val` is and then we make a decision - and we will consider both the (negative `val`, positive `address`) and (positive `val`, negative `address`) cases separately.

|          | floating point value | reinterpreted unsigned int value | reinterpreted signed int value |
| -------- | -------------------- | -------------------------------- | ------------------------------ |
| `val_1`  | __0.1f__             | __1036831949__                   | __1036831949__                 |
| `addr_1` | 0.2f                 | 1045220557                       | 1045220557                     |
| ...      | ...                  | ...                              | ...                            |
| `val_2`  | 0.2f                 | __1045220557__                   | 1045220557                     |
| `addr_2` | __-0.2f__            | 3192704205                       | __-1102263091__                |
| ...      | ...                  | ...                              | ...                            |
| `val_3`  | __-0.2f__            | 3192704205                       | __-1102263091__                |
| `addr_3` | 0.2f                 | __1045220557__                   | 1045220557                     |
| ...      | ...                  | ...                              | ...                            |
| `val_4`  | __-0.2f__            | 3192704205                       | -1102263091                    |
| `addr_4` | -0.1f                | __3184315597__                   | __-1110651699__                |

In the first 2 cases - where `val` is positive - we see that reinterpreting as a signed integer and then taking the _minimum_ matches our floating point minimum i.e. here we take the `atomicMin`.

In the next 2 cases - where `val` is negative - the above is inconsistent, and we should instead reinterpret as unsigned integers, where the floating point minimum now corresponds to the unsigned _maximum_ i.e. here we take the `atomicMax` of the integers, even though we were looking for the `atomicMin` of the floats!

Remember, in all our cases above, we are making the decision for the reinterpreted type _based solely on the sign of `val`_.

## Why this happens

This should be apparent when considering unsigned versus signed integers. For signed integers, we compare the trailing 31 bits (magnitude) in a way identical to unsigned integers, but if the sign bit is on (negative number) then a smaller magnitude implies a larger number, and vice versa.

When we first observe the sign of `val`, we don't know what the sign of the other operand at `address` is. But we can infer the following:

1. If `val` is positive and
	1. `address` is positive then taking the minimum of the signed or unsigned reinterpretation makes no difference. _Taking the maximum in either reinterpretation gives the wrong answer_.
	2. `address` is negative then we must take either the minimum of the signed reinterpretation or the maximum of the unsigned interpretation.
	3. __The consistent logic is thus taking the minimum of the signed int reinterpretation__.
2. If `val` is negative and
	1. `address` is positive then we must take either the minimum of the signed reinterpretation or the maximum of the unsigned interpretation.
	2. `address` is negative then taking the maximum of the signed or unsigned reinterpretation makes no difference. _Taking the minimum in either reinterpretation gives the wrong answer, since we need to look for the bigger magnitude in order to be more negative_.
	3. __The consistent logic is thus taking the maximum of the unsigned int reinterpretation__.

## A final note on signed zeroes

In the StackOverflow link, there is an older answer that used a simple `val >= 0` comparison to determine which logic branch to use. This works, except in some cases where `val == -0.0f`.

For example, if `val == -0.0f` and `*address == -1.0f`, then since `val >= 0` evaluates to `true`, one might attempt to use the minimum of the signed int reinterpretation as discussed above, evaluating `val` as `-2147483648`. This would, however, mark `-0.0f` as the minimum erroneously since `-1.0f` is `-1082130432`; indeed, since `val` is 'negative' we should have used the maximum of the unsigned int reinterpretation.

This is entirely due to zero being a special number in the exponent section (`0.0f` is just `0x00000000` and `-0.0f` is just `0x80000000`). The correction, as stated in the link, is to just read the bit directly, either via custom bit twiddling or CUDA's `signbit()`.
