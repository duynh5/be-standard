package product

type Product struct {
	ID   string
	SKU  string
	Name string
}

type Store struct {
	products map[string]Product
}

func NewStore() *Store {
	return &Store{products: make(map[string]Product)}
}
