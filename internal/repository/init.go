package repository

type Repositories struct {
	CoinsRepo CoinsRepository
	OrderRepo OrderRepository
}

func InitRepositories() *Repositories {
	return &Repositories{
		CoinsRepo: NewCoinsRepository(),
		OrderRepo: NewOrderRepository(),
	}
}
