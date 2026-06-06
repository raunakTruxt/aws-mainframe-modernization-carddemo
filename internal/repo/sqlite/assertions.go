package sqlite

import "github.com/aws-samples/aws-mainframe-modernization-carddemo/internal/repo"

var (
	_ repo.AccountRepository     = (*AccountStore)(nil)
	_ repo.CardRepository        = (*CardStore)(nil)
	_ repo.CustomerRepository    = (*CustomerStore)(nil)
	_ repo.CardXrefRepository    = (*CardXrefStore)(nil)
	_ repo.TransactionRepository = (*TransactionStore)(nil)
	_ repo.TranTypeRepository    = (*TranTypeStore)(nil)
	_ repo.TranCatRepository     = (*TranCatStore)(nil)
	_ repo.DiscGroupRepository   = (*DiscGroupStore)(nil)
	_ repo.TranCatBalRepository  = (*TranCatBalStore)(nil)
	_ repo.UserSecRepo           = (*UserSecStore)(nil)
)
