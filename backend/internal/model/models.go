package model

type UserContext struct {
	ID   string `json:"id"`
	Name string `json:"name"`
	Type string `json:"type"` // "admin" or "users"
}

type Book struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	Currency    string `json:"currency"`
	Description string `json:"description"`
	OwnerID     string `json:"owner_id"`
	CreatedAt   string `json:"created_at"`
	UpdatedAt   string `json:"updated_at"`
	Role        string `json:"role,omitempty"` // populated for query results
}

type BookMember struct {
	BookID    string `json:"book_id"`
	UserID    string `json:"user_id"`
	UserName  string `json:"user_name"`
	Role      string `json:"role"` // "owner", "editor", "viewer"
	CreatedAt string `json:"created_at"`
}

type Wallet struct {
	ID              string `json:"id"`
	BookID          string `json:"book_id"`
	Type            string `json:"type"` // wechat, alipay, debit, credit, cash, investment, other/custom
	Name            string `json:"name"`
	Balance         int64  `json:"balance"`      // Int64 in cents
	CreditLimit     int64  `json:"credit_limit"` // Int64 in cents
	BillingDay      int    `json:"billing_day"`
	RepaymentDay    int    `json:"repayment_day"`
	CardLastFour    string `json:"card_last_four"`
	ExcludeNetAsset bool   `json:"exclude_net_asset"`
	IsArchived      bool   `json:"is_archived"`
	CreatedAt       string `json:"created_at"`
	UpdatedAt       string `json:"updated_at"`
}

type WalletSummary struct {
	TotalAssets      int64 `json:"total_assets"`
	TotalLiabilities int64 `json:"total_liabilities"`
	NetAssets        int64 `json:"net_assets"`
}

type Category struct {
	ID        string  `json:"id"`
	BookID    string  `json:"book_id"`
	Type      string  `json:"type"` // expense, income
	Name      string  `json:"name"`
	Icon      string  `json:"icon"`
	Color     string  `json:"color"`
	ParentID  *string `json:"parent_id"`
	CreatedAt string  `json:"created_at"`
}

type Tag struct {
	ID        string `json:"id"`
	BookID    string `json:"book_id"`
	Name      string `json:"name"`
	Color     string `json:"color"`
	CreatedAt string `json:"created_at"`
}

type Transaction struct {
	ID          string   `json:"id"`
	BookID      string   `json:"book_id"`
	Type        string   `json:"type"` // expense, income, transfer
	Amount      int64    `json:"amount"` // Int64 cents
	Date        string   `json:"date"`   // YYYY-MM-DD
	WalletID    string   `json:"wallet_id"`
	ToWalletID  *string  `json:"to_wallet_id"`
	CategoryID  *string  `json:"category_id"`
	Note        string   `json:"note"`
	OrderNo     string   `json:"order_no"`
	CreatedBy   string   `json:"created_by"`
	CreatedAt   string   `json:"created_at"`
	UpdatedAt   string   `json:"updated_at"`
	Tags        []Tag    `json:"tags"`
	Attachments []string `json:"attachments,omitempty"`
	CategoryName  string `json:"category_name,omitempty"`
	CategoryIcon  string `json:"category_icon,omitempty"`
	CategoryColor string `json:"category_color,omitempty"`
	WalletName    string `json:"wallet_name,omitempty"`
	ToWalletName  string `json:"to_wallet_name,omitempty"`
}

type DailySummary struct {
	Date        string `json:"date"`
	BookID      string `json:"book_id"`
	Type        string `json:"type"`
	CategoryID  string `json:"category_id"`
	TotalAmount int64  `json:"total_amount"`
	Count       int    `json:"count"`
}

type MonthlyReport struct {
	Year         int                  `json:"year"`
	Month        int                  `json:"month"`
	TotalExpense int64                `json:"total_expense"`
	TotalIncome  int64                `json:"total_income"`
	NetSavings   int64                `json:"net_savings"`
	Categories   []CategoryReportItem `json:"categories"`
}

type CategoryReportItem struct {
	CategoryID    string  `json:"category_id"`
	CategoryName  string  `json:"category_name"`
	CategoryIcon  string  `json:"category_icon"`
	CategoryColor string  `json:"category_color"`
	Amount        int64   `json:"amount"`
	TotalAmount   int64   `json:"total_amount,omitempty"`
	Count         int     `json:"count"`
	Percentage    float64 `json:"percentage"`
}

type TrendMonthItem struct {
	Month   string `json:"month"`
	Expense int64  `json:"expense"`
	Income  int64  `json:"income"`
}

type TrendReport struct {
	Months []TrendMonthItem `json:"months"`
}

type DailyReportItem struct {
	Date    string `json:"date"`
	Expense int64  `json:"expense"`
	Income  int64  `json:"income"`
}

type Snapshot struct {
	ID        string `json:"id"`
	FileName  string `json:"file_name"`
	FilePath  string `json:"file_path"`
	FileSize  int64  `json:"file_size"`
	SHA256    string `json:"sha256"`
	CreatedBy string `json:"created_by"`
	CreatedAt string `json:"created_at"`
}

type ApiResponse struct {
	Code    int         `json:"code"`
	Message string      `json:"message"`
	Data    interface{} `json:"data,omitempty"`
}
