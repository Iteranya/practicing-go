package inventory

type Inventory struct {
	Id     int
	Slug   string
	Name   string
	Desc   string
	Tag    string
	Label  string
	Stock  int64
	Custom map[string]any
}
