package main

type Product struct { // This can be a single product, or a package of product
	Id     int
	Slug   string
	Name   string
	Desc   string
	Tag    string
	Label  string
	Price  int64
	Avail  bool
	Items  *[]string       // This is an array of slug that this uses. Optional (Say, like, a morning package, has coffee and croissant)
	Recipe *map[string]int // This is the slug of stock in inventory and how much it uses. Optional (Say, 5 grams coffee, 200 ml milk)
	Custom map[string]any
}
