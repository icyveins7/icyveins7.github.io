---
title: "CRTP, method chaining, and static polymorphism"
date: 2024-05-04T20:00:00+08:00
draft: false
toc: false
images:
tags:
  - "c++"
  - "crtp"
---

> Yes, it's yet another blogpost about CRTP and how it'd be useful..

# The 1st issue: method chaining
Have you seen a billion other blogposts about CRTP? Yes, so have I. But maybe there's a reason for all of them; it wasn't really apparent when reading them previously why it would be useful and/or why I would ever need it. 


But recently, while writing some simple templated code for [```ufl```](https://github.com/icyveins7/ufl), I had the bright idea of trying to make method chaining possible for the class. That's when my templated class and its derived friend implementation started to fall apart. I fixed this by using CRTP for the first time.

I think [this post](https://dariusgrant.github.io/2021/02/11/Method-Chaining-Base-Class-Methods-With-Derived-Classes.html) is probably the best explanation for how CRTP solves this problem, so I'm not going to repeat it here, but rather leave a link (for myself). Bonus points for not having ads all over the place on that page.

# The 2nd issue: static polymorphism

I remember stumbling on a [video](https://www.youtube.com/watch?v=NH1Tta7purM) that removed virtual functions with templates a few years back. At the time, I didn't really grasp the concept very well (_is this what they call experience?_), but it's starting to dawn on me I think.

While writing some code for [```uhdeb```](https://github.com/icyveins7/uhdeb), I tried to wrap 2 similar but related streamer object classes in a container. I wanted a parent class that would hold a different type of streamer object - a ```tx_streamer``` or a ```rx_streamer``` - and to perform this initialization in the constructor (using another common object type that was passed in).

My first thought was to define a parent templated class that contained a ```T m_stream```. But now I had 2 choices:

1. I write a custom constructor for each of the derived classes. This would call the appropriate initialization for each type of ```m_stream```. But I wanted the constructor to also do similar things for both derived classes - allocate some memory, start a thread - and so I would have had to copy all those calls, violating DRY.
2. Do some virtual calls.

The code (roughly speaking) looked like this:

```c++
template <typename T>
class Parent
{
    ...

    T m_stream;
};

class DerivedRX : Parent<RXStreamer>
{
    DerivedRX(...) : Parent(...)
    {
        // create the RXStreamer..
        ...

        // the rest depend on the streamer, so I couldn't throw it into Parent's constructor
        allocate();
        start_thread();
    }
};

class DerivedTX : Parent<TXStreamer>
{
    DerivedTX(...) : Parent(...)
    {
        // create the TXStreamer..
        ...

        // the rest depend on the streamer, so I couldn't throw it into Parent's constructor
        allocate();
        start_thread();
    }
};
```

Could I have refactored the code to not have this problem? Probably. But at the time I was adamant on finding a way of getting this to work.

The way I did it was to push all the 'standard' constructor logic to the parent class. The parent class would call the appropriate derived class's ```create_stream``` method through CRTP:

```c++
template <typename T, typename U>
class Parent
{
    Parent()
    {
        static_cast<U*>(this)->create_stream();

        // the rest of the ctor..

    }


    T m_stream;
};

class DerivedRX : Parent<RXStreamer, DerivedRX>
{
    DerivedRX(...) : Parent(...)
    {
        // nothing else needs to be done..
    }

    void create_stream()
    {
        // custom RX code..
    }
};

class DerivedTX : Parent<TXStreamer, DerivedTX>
{
    DerivedTX(...) : Parent(...)
    {
        // nothing else needs to be done
    }

    void create_stream()
    {
        // custom TX code..
    }
};
```

This avoids any use of ```virtual``` methods - _no vtables here hoho_ - and the correct derived method is called for each derived class!

Oh, I also had to add a ```friend class``` declaration within the derived class definitions in order for it to work.

Now, if you look at this and are thinking: couldn't he have just moved the custom streamer types back into the derived class definitions? Then there would be no need for any of these shenanigans, and there would just be simple parent and derived classes. And you would be right.

But like I said, I was exploring, and it was interesting.

Going to leave some more references here for myself:

1. [https://www.fluentcpp.com/2017/05/16/what-the-crtp-brings-to-code/](https://www.fluentcpp.com/2017/05/16/what-the-crtp-brings-to-code/)
2. [https://eli.thegreenplace.net/2013/12/05/the-cost-of-dynamic-virtual-calls-vs-static-crtp-dispatch-in-c](https://eli.thegreenplace.net/2013/12/05/the-cost-of-dynamic-virtual-calls-vs-static-crtp-dispatch-in-c)

I'm sure some day soon I'll find a stronger use-case for this. I never really liked doing ```virtual``` methods after all.
