---
title: "Build your own tools, gcc edition"
date: 2025-10-20T20:00:00+08:00
draft: false
toc: false
images:
tags:
  - "gcc"
  - "compiler"
---

> Not complicated, but always good to know the process.


In this series (?) I'll cover something I always wanted to write down: compiling the toolchains/compilers I use from source. This means, specifically, no using of package managers i.e. not allowed to `sudo apt install` dependencies. Of course, this has its limits (*you need a compiler to compile gcc, and some OSes don't come pre-installed with one*). However, the premise here is to at least be familiar with the minimal required dependencies, and where to get them (at the time of writing), and then what to do to build everything.

While this seems like a pointless exercise, doing this at least once comes in handy when you have to

- install something without sudo rights (because let me compile things in my user space if I want to damn it)
- install something on an offline machine
- install a version that doesn't (easily) exist on your system's package manager (because of reasons like an outdated OS, which happens way more often than you think on workplace servers)
- or any other equivalent reason

For the above reasons, none of the steps described in this post or later posts will require administrator privileges; everything will be installed into your `home` directory (and with that you will have to set things up yourself after).

We begin, as we should, with the granddaddy of all tools: `gcc`.

# Installing gcc-11.5 on Ubuntu 22.04

The default repositories for Ubuntu 22.04 do not contain gcc-11.5, so we begin by reading its [documentation](https://gcc.gnu.org/install/).

Here we are presented with the essential dependencies:

- [GMP]([https://gmplib.org](https://gmplib.org/))
- [MPFR]([https://www.mpfr.org](https://www.mpfr.org/)
- [MPC]([https://www.multiprecision.org](https://www.multiprecision.org/)

After downloading all of them, we must install them in the above order. This is because they are recursively dependent i.e MPC depends on MPFR which depends on GMP.

# A primer on autoconf-style builds

> Feel free to skip this if you're already familiar with it.

Every tool that is linux-based will almost certainly include an `autoconf` script. This comes in the form of a `configure`script.

For most scenarios, running it without any extra options would suffice. Hence, other than some variations with extra steps like for testing, almost everything will build via 

```bash
./configure
make
sudo make install
```

What each line does, very briefly, is

1. Configures the build and generates (optimized) makefiles. This usually exists to somewhat tailor the compilation to your system and/or detect missing dependencies.
2. Compiles the thing.
3. Installs the thing; *read: this means it copies the compiled executables/libraries to the designated path, which by default is a system path like `/usr/local`, hence the requirement that you do it with `sudo`. It may also help you with setting up reasonable symlinks*.

# A bit of guidance

While the instructions on the `gcc` site are pretty comprehensive, I'll tailor a few steps here to make sure we can build and use `gcc` without `sudo`.

1. Install GMP to a specific subdirectory in home. Example:
```bash
export GMPDIR=/home/username/gmp
tar -xzvf gmp-...tar.gz
cd gmp-...
./configure --prefix=$GMPDIR
make
make install
```

2. Install MPFR to a specific subdirectory in home, pointing to our GMP install prefix. Example:
```bash
export MPFRDIR=/home/username/mpfr
tar -xzvf mpfr-...tar.gz
cd mpfr-...
./configure --prefix=$MPFRDIR --with-gmp=$GMPDIR
make
make install
```

3. Install MPC to a specific subdirectory in home, pointing to the two previous install suffixes. Example:
```bash
export MPCDIR=/home/username/mpc
tar -xzvf mpc-...tar.gz
cd mpc-...
./configure --prefix=$MPCDIR --with-gmp=$GMPDIR --with-mpfr=$MPFRDIR
make
make install
```

4. Now we can build gcc using these. Somehow, I wrote down that I had to define the library paths explicitly *before* this, so I'll include it here as well:

```bash
export LD_LIBRARY_PATH=$GMPDIR/lib:$MPFRDIR/lib:$MPCDIR/lib:$LD_LIBRARY_PATH
# gcc recommends you to
../configure --prefix=/opt/gcc-11.5.0 --with-gmp=$GMPDIR --with-mpfr=$MPFRDIR --with-mpc=$MPCDIR
```

5. After this it's simply building and installing as per usual:

```bash
make # with -j whatever if you want
make install
```

6. The final step is to make sure it runs (instead of your system's default version):

```bash
export PATH=/opt/gcc-11.5.0/bin:$PATH
export LD_LIBRARY_PATH=/opt/gcc-11.5.0/lib:$LD_LIBRARY_PATH
# I usually chuck these into my ~/.bashrc
# and disable it if i want to switch back to the system version.
# Kinda like a manual way of handling alternatives.
```

And that's all.


