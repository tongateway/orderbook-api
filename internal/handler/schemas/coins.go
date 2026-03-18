package schemas

type CoinRequestHTTP struct {
	Id uint64 `uri:"id" binding:"required"`
}

type CoinListRequestHTTP struct {
	Offset int64  `form:"offset"`
	Limit  int64  `form:"limit"`
	Sort   string `form:"sort"`
	Order  string `form:"order"`
}
