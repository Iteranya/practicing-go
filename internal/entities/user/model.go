package user

type User struct {
	Id          int
	Username    string
	DisplayName string
	Hash        string
	Role        string // Slug of Role
	Active      bool
	Setting     map[string]any
	Custom      map[string]any
}
