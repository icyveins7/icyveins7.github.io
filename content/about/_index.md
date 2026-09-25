+++
title = "About"
+++

<style>
.about {
  display: flex;
  gap: 3rem;
  align-items: flex-start;
}

.about-image {
  flex: 0 0 150px;
  text-align: center;
}

.about-image img {
  width: 150px;
  border-radius: 50%;
}

.about-image h3 {
  display: block;
  text-align: center;
}

@media (max-width: 600px) {
  .about {
    flex-direction: column;
  }

  .about-image {
    align-self: center;
  }
}

hr {
  margin-bottom: 2rem;
}
</style>
<hr>
<div class="about">
  <div class="about-image">
    <img src="/img/cat.png">
    <h3>icyveins7</h3>
    <p>Physicist, Engineer</p>
    {{< github url="https://github.com/icyveins7" >}}
  </div>

  <div class="about-content">
    <p>
        Physicist by training; digital signal (and now image!) processing engineer by craft.
        Loves writing software in C++ and Python, and custom CUDA kernels.
    </p>
  </div>
</div>
