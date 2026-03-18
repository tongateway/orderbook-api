package schemas

type VaultRequestHTTP struct {
	Id uint64 `uri:"id" binding:"required"`
}

type VaultListRequestHTTP struct {
	Offset              int64  `form:"offset"`
	Limit               int64  `form:"limit"`
	Sort                string `form:"sort"`
	Order               string `form:"order"`
	JettonMinterAddress string `form:"jetton_minter_address"`
	Type                string `form:"type"` // jetton, ton
}
