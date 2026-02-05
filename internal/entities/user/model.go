package main

type User struct {
	Id          int
	Username    string
	DisplayName string
	Hash        string
	Role        string
	Active      bool
	Setting     map[string]any
	Custom      map[string]any
}
