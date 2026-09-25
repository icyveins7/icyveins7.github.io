---
title: "Getting rid of clangd's errors on Windows"
date: 2024-03-26T20:00:00+08:00
draft: false
toc: false
images:
tags:
  - "neovim"
  - "clangd"
  - "ninja"
---

> My first steps into migrating to neovim, some clangd problems and my solutions..

# Kickstart.nvim brought me here

I started using vim motions in VSCode a month or two ago, and decided to actually try to see if neovim would work for me. Honestly, I probably wouldn't have gone down this path if [kickstart.nvim](https://github.com/nvim-lua/kickstart.nvim) didn't exist. But it does, and I tried it, and it looks like I'm here to stay.

# clangd as my first LSP

In kickstart.nvim's template, ```clangd``` is listed as an example LSP, so I decided to uncomment that line and try it out. I opened my [ffs](https://github.com/icyveins7/ffs) repository at the time, and was immediately greeted with a flood of warnings on my code. This brings us to the first problem.

# Header Resolution with clangd

My code was immediately flooded with warnings, to the point that ```clangd``` itself said it would stop reporting errors. A lot of them looked like this:

![clangd-warnings](/static/images/clangd-warnings.png)

It seemed like most of them were related to header resolution; I had seen a similar type of error/warning in VSCode before, which always seemed to magically disappear after running ```cmake``` the first time.

Solution? Generate a ```compile_commands.json``` [(see this)](https://clangd.llvm.org/design/compile-commands). In particular, I needed to make sure this was _near the root of the directory_. In my original setup, I had separate ```CMakeLists.txt``` files in my ```examples/``` and ```tests/``` subdirectories, and would manually enter each subdirectory when I needed to do stuff there. However, this meant that when opening my ```include/``` subdirectory's files, I would be greeted with similar ```clangd``` errors, as it wouldn't see the ```compile_commands.json``` generated from running ```cmake``` for the prior folders.

So I needed to write an outermost ```CMakeLists.txt```, to ensure that the ```compile_commands.json``` appeared in the ```build/``` subdirectory at the top. Enabling this explicitly is trivial by just placing this line in that top-level ```CMakeLists.txt```:

```
set(CMAKE_EXPORT_COMPILE_COMMANDS ON)
```

# Compile commands don't work with MSVC

The second problem is related to the first. As it turns out, [setting the above does absolutely nothing when using MSVC](https://cmake.org/cmake/help/latest/variable/CMAKE_EXPORT_COMPILE_COMMANDS.html), which I was using since I was in Windows. I didn't want to use MinGW-gcc here, so I thought I'd try the other generator in the link: [Ninja](https://github.com/ninja-build/ninja).

After installing and calling ```cmake .. -G Ninja```, the ```compile_commands.json``` was generated and everything worked as they should.

# Other Ninja Benefits

1. I always didn't like how MSVC _had_ to be different by pushing the build type configuration to after the ```cmake``` call. Using ```ninja``` lets me have a more symmetric experience while retaining the use of MSVC, as now I can always specify ```-DCMAKE_BUILD_TYPE``` at the ```cmake``` step regardless of whether I'm on unix or Windows.
2. I get to ```make``` things by ```ninja``` and ```make clean``` by ```ninja clean```. This is pretty satisfying.
3. Having ```compile_commands.json``` means I can see the literal command for every executable, rather than having to dig inside the ```.vcxproj``` to double-check every option that was enabled; previously I would just wait til compile time to see the command line arguments the project file would use.


