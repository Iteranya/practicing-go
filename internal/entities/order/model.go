package main

type Order struct {
	Id      int
	Items   []string // Slug of Products Bought
	ClerkId int      // User ID of the Cashier
	Total   int64    // Total Price
	Paid    int64    // Paid
	Change  int64    // Change
	Created int64    // Created
	Custom  map[string]any
}
