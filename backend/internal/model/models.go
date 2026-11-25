package model

import "encoding/json"

type UserContext struct {
	ID    string `json:"id"`
	Name  string `json:"name"`
	Type  string `json:"type"` // "admin" or "users"
	Email string `json:"email,omitempty"`
	Role  string `json:"role,omitempty"` // "admin" or "user"
}

func (u *UserContext) IsAdmin() bool {
	if u == nil {
		return false
	}
	return u.Role == "admin" || u.Type == "admin"
}

type User struct {
	ID            string `json:"id"`
	Username      string `json:"username"`
	Name          string `json:"name"`
	Email         string `json:"email"`
	PasswordHash  string `json:"-"`
	Avatar        string `json:"avatar"`
	Role          string `json:"role"`
	EmailBound    bool   `json:"email_bound"`
	GatewayUID    string `json:"gateway_uid,omitempty"`
	DefaultBookID string `json:"default_book_id,omitempty"`
	DailyAiLimit  int    `json:"daily_ai_limit"`
	IsActive      bool   `json:"is_active"`
	LastActiveAt  string `json:"last_active_at"`
	CreatedAt     string `json:"created_at"`
}

type UserProfile struct {
	UserID                 string `json:"user_id"`
	Username               string `json:"username"`
	UserName               string `json:"user_name"`
	UserType               string `json:"user_type"`
	Email                  string `json:"email"`
	Avatar                 string `json:"avatar"`
	Role                   string `json:"role"`
	EmailBound             bool   `json:"email_bound"`
	GatewayBound           bool   `json:"gateway_bound"`
	DefaultBookID          string `json:"default_book_id,omitempty"`
	DailyAiLimit           int    `json:"daily_ai_limit"`
	TodayAiCount           int    `json:"today_ai_count"`
	StreakDays             int    `json:"streak_days"`
	RecordedDays           int    `json:"recorded_days"`
	TotalTransactionsCount int    `json:"total_transactions_count"`
	CreatedBooksCount      int    `json:"created_books_count"`
	JoinedBooksCount       int    `json:"joined_books_count"`
	TotalBooksCount        int    `json:"total_books_count"`
	TotalWalletsCount      int    `json:"total_wallets_count"`
	HealthScore            int    `json:"health_score"`
	HealthLabel            string `json:"health_label"`
	SavingsRate            int    `json:"savings_rate"`
}

type Book struct {
	ID          string       `json:"id"`
	Name        string       `json:"name"`
	Currency    string       `json:"currency"`
	Description string       `json:"description"`
	Color       string       `json:"color"`
	Budget      int64        `json:"budget"`
	InviteCode  string       `json:"invite_code"`
	OwnerID     string       `json:"owner_id"`
	CreatedAt   string       `json:"created_at"`
	UpdatedAt   string       `json:"updated_at"`
	Role        string       `json:"role,omitempty"`
	IsDefault   bool         `json:"is_default"`
	TxCount     int          `json:"tx_count"`
	Members     []BookMember `json:"members,omitempty"`
}

type BookMember struct {
	BookID    string `json:"book_id"`
	UserID    string `json:"user_id"`
	UserName  string `json:"user_name"`
	Role      string `json:"role"` // "owner", "editor", "viewer"
	Avatar    string `json:"avatar"`
	Color     string `json:"color"`
	IsVirtual bool   `json:"is_virtual"`
	Status    string `json:"status"`
	IsMe      bool   `json:"is_me"`
	CreatedAt string `json:"created_at"`
}

type Wallet struct {
	ID              string `json:"id"`
	BookID          string `json:"book_id"`
	Type            string `json:"type"` // wechat, alipay, debit, credit, cash, investment, other/custom
	Name            string `json:"name"`
	GroupName       string `json:"group_name"`
	Color           string `json:"color"`
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
	BgColor   string  `json:"bg_color"`
	ParentID  *string `json:"parent_id"`
	IsCustom  bool    `json:"is_custom"`
	SortOrder int     `json:"sort_order"`
	// UsageCount 是该分类近 90 天在本账本内的使用笔数（一级含其二级合计），
	// 仅记账选分类时用于“常用在前”的动态排序，不参与分类管理里的手动顺序。
	UsageCount int    `json:"usage_count,omitempty"`
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
	ID              string   `json:"id"`
	BookID          string   `json:"book_id"`
	Type            string   `json:"type"` // expense, income, transfer
	Amount          int64    `json:"amount"`
	Date            string   `json:"date"` // YYYY-MM-DD
	TxTime          string   `json:"tx_time"`
	WalletID        string   `json:"wallet_id"`
	ToWalletID      *string  `json:"to_wallet_id"`
	CategoryID      *string  `json:"category_id"`
	Title           string   `json:"title"`
	SubCategory     string   `json:"sub_category"`
	Note            string   `json:"note"`
	OrderNo         string   `json:"order_no"`
	IncludeBudget   bool     `json:"include_budget"`
	Reimburse       bool     `json:"reimburse"`
	CreatedBy       string   `json:"created_by"`
	CreatedByName   string   `json:"created_by_name,omitempty"`
	CreatedByAvatar string   `json:"created_by_avatar,omitempty"`
	// MemberID 是这笔流水的归属成员（真实成员或虚拟成员），与创建人可以不同；
	// 为空时等同于创建人（历史数据回填同样处理）。
	MemberID     string `json:"member_id"`
	MemberName   string `json:"member_name,omitempty"`
	MemberAvatar string `json:"member_avatar,omitempty"`
	MemberColor  string `json:"member_color,omitempty"`
	CreatedAt       string   `json:"created_at"`
	UpdatedAt       string   `json:"updated_at"`
	Tags            []Tag    `json:"tags"`
	Attachments     []string `json:"attachments,omitempty"`
	CategoryName    string   `json:"category_name,omitempty"`
	CategoryIcon    string   `json:"category_icon,omitempty"`
	CategoryColor   string   `json:"category_color,omitempty"`
	CategoryBgColor string   `json:"category_bg_color,omitempty"`
	WalletName      string   `json:"wallet_name,omitempty"`
	ToWalletName    string   `json:"to_wallet_name,omitempty"`
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
	BgColor       string  `json:"bg_color,omitempty"`
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

type StatsCategoryItem struct {
	CategoryID    string `json:"category_id"`
	CategoryName  string `json:"category_name"`
	CategoryIcon  string `json:"category_icon"`
	CategoryColor string `json:"category_color"`
	BgColor       string `json:"bg_color"`
	Amount        int64  `json:"amount"`
	Percentage    int    `json:"percentage"`
	Count         int    `json:"count"`
}

type StatsSeriesItem struct {
	Date    string `json:"date"`
	Label   string `json:"label"`
	Expense int64  `json:"expense"`
	Income  int64  `json:"income"`
}

type StatsReport struct {
	Period          string              `json:"period"`
	Type            string              `json:"type"`
	StartDate       string              `json:"start_date"`
	EndDate         string              `json:"end_date"`
	TotalExpense    int64               `json:"total_expense"`
	TotalIncome     int64               `json:"total_income"`
	NetSavings      int64               `json:"net_savings"`
	SavingsRate     int                 `json:"savings_rate"`
	Budget          int64               `json:"budget"`
	BudgetUsed      int64               `json:"budget_used"`
	Categories      []StatsCategoryItem `json:"categories"`
	Members         []StatsMemberItem   `json:"members"`
	DailySeries     []StatsSeriesItem   `json:"daily_series"`
	TopTransactions []Transaction       `json:"top_transactions"`
}

// StatsMemberItem 是统计里按成员聚合的一行：该成员在所选周期/类型下的合计。
type StatsMemberItem struct {
	MemberID    string `json:"member_id"`
	MemberName  string `json:"member_name"`
	MemberAvatar string `json:"member_avatar"`
	MemberColor string `json:"member_color"`
	IsVirtual   bool   `json:"is_virtual"`
	Amount      int64  `json:"amount"`
	Percentage  int    `json:"percentage"`
	Count       int    `json:"count"`
}

// RangeStats 是 Agent query_stats 工具的返回结构：任意起止日期区间的汇总。
type RangeMonthItem struct {
	Month   string `json:"month"`
	Expense int64  `json:"expense"`
	Income  int64  `json:"income"`
}

type RangeStats struct {
	StartDate    string              `json:"start_date"`
	EndDate      string              `json:"end_date"`
	TotalExpense int64               `json:"total_expense"`
	TotalIncome  int64               `json:"total_income"`
	NetSavings   int64               `json:"net_savings"`
	Categories   []StatsCategoryItem `json:"categories"`
	Monthly      []RangeMonthItem    `json:"monthly"`
}

// MemberTotal 是某成员在一段时间内的收支合计。
type MemberTotal struct {
	UserID  string `json:"user_id"`
	Expense int64  `json:"expense"`
	Income  int64  `json:"income"`
	Count   int    `json:"count"`
}

type CategoryBudgetItem struct {
	CategoryID      string `json:"category_id"`
	CategoryName    string `json:"category_name"`
	CategoryIcon    string `json:"category_icon"`
	CategoryColor   string `json:"category_color"`
	BgColor         string `json:"bg_color"`
	LimitAmount     int64  `json:"limit_amount"`
	SpentAmount     int64  `json:"spent_amount"`
	RemainingAmount int64  `json:"remaining_amount"`
	Percentage      int    `json:"percentage"`
}

type BudgetOverview struct {
	BookID          string               `json:"book_id"`
	Year            int                  `json:"year"`
	Month           int                  `json:"month"`
	TotalBudget     int64                `json:"total_budget"`
	TotalSpent      int64                `json:"total_spent"`
	RemainingBudget int64                `json:"remaining_budget"`
	RemainingDays   int                  `json:"remaining_days"`
	DailyAllowance  int64                `json:"daily_allowance"`
	Percentage      int                  `json:"percentage"`
	Categories      []CategoryBudgetItem `json:"categories"`
}

type RecurringRule struct {
	ID            string `json:"id"`
	BookID        string `json:"book_id"`
	Type          string `json:"type"`
	CategoryID    string `json:"category_id"`
	CategoryName  string `json:"category_name"`
	CategoryIcon  string `json:"category_icon"`
	CategoryColor string `json:"category_color"`
	BgColor       string `json:"bg_color"`
	SubCategory   string `json:"sub_category"`
	Title         string `json:"title"`
	Amount        int64  `json:"amount"`
	Cycle         string `json:"cycle"`
	StartDate     string `json:"start_date"`
	EndRule       string `json:"end_rule"`
	NextDate      string `json:"next_date"`
	ExecutedCount int    `json:"executed_count"`
	WalletID      string `json:"wallet_id"`
	WalletName    string `json:"wallet_name"`
	ExcludeStats  bool   `json:"exclude_stats"`
	IsActive      bool   `json:"is_active"`
	CreatedBy     string `json:"created_by"`
	CreatedAt     string `json:"created_at"`
	UpdatedAt     string `json:"updated_at"`
}

type AiDraft struct {
	Type          string `json:"type"`
	Amount        int64  `json:"amount"`
	Title         string `json:"title"`
	Note          string `json:"note"`
	CategoryID    string `json:"category_id"`
	CategoryName  string `json:"category_name"`
	SubCategory   string `json:"sub_category"`
	CategoryIcon  string `json:"category_icon"`
	CategoryColor string `json:"category_color"`
	BgColor       string `json:"bg_color"`
	WalletID      string `json:"wallet_id"`
	WalletName    string `json:"wallet_name"`
	Date          string `json:"date"`
}

type AiUsageSummary struct {
	TodayCount int `json:"today_count"`
	DailyLimit int `json:"daily_limit"`
	Remaining  int `json:"remaining"`
}

// AiChatMessage 是存到服务端的 AI 聊天记录：按用户 + 账本隔离，
// 各人只看自己的对话。Draft 存原始 JSON（前端的草稿字段可能比后端
// 的 AiDraft 多，如 member_id），读出时原样返回，不做结构校验。
// ImagePath 只存 ai-images 目录下的文件名，不存绝对路径；文件本身
// 存在 <dataDir>/ai-images/，随 UGAPP_DATA_DIR 走，Docker 部署时
// 挂一个数据卷即可一起带走。
type AiChatMessage struct {
	ID         string          `json:"id"`
	BookID     string          `json:"book_id"`
	UserID     string          `json:"user_id,omitempty"`
	Role       string          `json:"role"`
	Text       string          `json:"text"`
	IsVoice    bool            `json:"is_voice"`
	ImagePath  string          `json:"image_path,omitempty"`
	ImageLabel string          `json:"image_label,omitempty"`
	OcrText    string          `json:"ocr_text,omitempty"`
	Draft      json.RawMessage `json:"draft,omitempty"`
	Saved      bool            `json:"saved"`
	SavedTxID  string          `json:"saved_tx_id,omitempty"`
	CreatedAt  string          `json:"created_at,omitempty"`
}

type AiInsightTip struct {
	Tag   string `json:"tag"`
	Color string `json:"color"`
	Bg    string `json:"bg"`
	Title string `json:"title"`
	Desc  string `json:"desc"`
}

type AiInsightsResponse struct {
	Summary string         `json:"summary"`
	Tips    []AiInsightTip `json:"tips"`
}

type AiUsageLog struct {
	ID           string `json:"id"`
	UserID       string `json:"user_id"`
	UserName     string `json:"user_name"`
	Action       string `json:"action"`
	Model        string `json:"model"`
	InputTokens  int64  `json:"input_tokens"`
	OutputTokens int64  `json:"output_tokens"`
	CreatedAt    string `json:"created_at"`
}

type AiConfig struct {
	Enabled         bool    `json:"enabled"`
	Provider        string  `json:"provider"`
	BaseURL         string  `json:"base_url"`
	APIKey          string  `json:"api_key"`
	HasAPIKey       bool    `json:"has_api_key"`
	Model           string  `json:"model"`
	SttBaseURL      string  `json:"stt_base_url"`
	SttAPIKey       string  `json:"stt_api_key"`
	HasSttAPIKey    bool    `json:"has_stt_api_key"`
	SttModel        string  `json:"stt_model"`
	// VisionModel 是图片识别专用的视觉模型。留空时用 Model。聊天模型大多是纯文本模型，
	// 看不懂图片，用它做 OCR 会失败或乱猜；视觉识别请在这里填支持图片输入的多模态模型。
	VisionModel     string  `json:"vision_model"`
	DailyLimit      int     `json:"daily_limit"`
	Temperature     float64 `json:"temperature"`
	OcrEnabled      bool    `json:"ocr_enabled"`
	InsightsEnabled bool    `json:"insights_enabled"`
}

type RegConfig struct {
	AllowRegister       bool   `json:"allow_register"`
	RequireInviteCode   bool   `json:"require_invite_code"`
	RequireEmailVerify  bool   `json:"require_email_verify"`
	InviteCode          string `json:"invite_code"`
	DefaultDailyAiLimit int    `json:"default_daily_ai_limit"`
}

type SystemConfig struct {
	AutoBackupEnabled bool   `json:"auto_backup_enabled"`
	AutoBackupCycle   string `json:"auto_backup_cycle"`
	AutoBackupRetain  int    `json:"auto_backup_retain"`
	BackupPath        string `json:"backup_path"`
	SmtpHost          string `json:"smtp_host"`
	SmtpPort          int    `json:"smtp_port"`
	SmtpUser          string `json:"smtp_user"`
	SmtpPass          string `json:"smtp_pass"`
	HasSmtpPass       bool   `json:"has_smtp_pass"`
	SmtpFrom          string `json:"smtp_from"`
	SmtpTLS           bool   `json:"smtp_tls"`
	SiteName          string `json:"site_name"`
}

type AdminUserItem struct {
	ID           string `json:"id"`
	Name         string `json:"name"`
	Email        string `json:"email"`
	Avatar       string `json:"avatar"`
	Role         string `json:"role"`
	IsActive     bool   `json:"is_active"`
	DailyAiLimit int    `json:"daily_ai_limit"`
	TodayAiCount int    `json:"today_ai_count"`
	BooksCount   int    `json:"books_count"`
	TxCount      int    `json:"tx_count"`
	InputTokens  int64  `json:"input_tokens"`
	OutputTokens int64  `json:"output_tokens"`
	LastActiveAt string `json:"last_active_at"`
}

type AdminDailyActivity struct {
	Date    string `json:"date"`
	Label   string `json:"label"`
	TxCount int    `json:"tx_count"`
	AiCalls int    `json:"ai_calls"`
}

type AdminOverview struct {
	TotalUsers             int                  `json:"total_users"`
	ActiveUsers            int                  `json:"active_users"`
	ActiveUsers7d          int                  `json:"active_users_7d"`
	TotalBooks             int                  `json:"total_books"`
	TotalTransactionsCount int                  `json:"total_transactions_count"`
	TodayAiCalls           int                  `json:"today_ai_calls"`
	TotalAiCalls           int                  `json:"total_ai_calls"`
	TotalInputTokens       int64                `json:"total_input_tokens"`
	TotalOutputTokens      int64                `json:"total_output_tokens"`
	TotalTokens            int64                `json:"total_tokens"`
	AiProvider             string               `json:"ai_provider"`
	AiModel                string               `json:"ai_model"`
	DailyActivity          []AdminDailyActivity `json:"daily_activity"`
	UserStats              []AdminUserItem      `json:"user_stats"`
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

// BookActivity 是账本动态的一条记录：谁、在哪个账本、干了什么。
// Summary 是展示给成员看的一句话（如“记了一笔午餐 ¥32.00”）；
// AmountCents/TxID 仅交易类动态有值。
type BookActivity struct {
	ID          string  `json:"id"`
	BookID      string  `json:"book_id"`
	UserID      string  `json:"user_id"`
	ActorName   string  `json:"actor_name"`
	Action      string  `json:"action"`
	Summary     string  `json:"summary"`
	AmountCents *int64  `json:"amount_cents,omitempty"`
	TxID        *string `json:"tx_id,omitempty"`
	CreatedAt   string  `json:"created_at"`
}

// 动态 action 取值（与 budget-alert 共用同一张表）。
const (
	ActivityTxCreated        = "transaction_created"
	ActivityTxUpdated        = "transaction_updated"
	ActivityTxDeleted        = "transaction_deleted"
	ActivityImportCommitted  = "import_committed"
	ActivityRecurringPosted  = "recurring_posted"
	ActivityRecurringChanged = "recurring_changed"
	ActivityWalletChanged    = "wallet_changed"
	ActivityBudgetChanged    = "budget_changed"
	ActivityCategoryChanged  = "category_changed"
	ActivityMemberJoined     = "member_joined"
	ActivityMemberRemoved    = "member_removed"
	ActivityBookRenamed      = "book_renamed"
	ActivityBookUpdated      = "book_updated"
	ActivityBudgetAlert      = "budget_alert"
)
