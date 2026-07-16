package clientauth

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"time"

	"gorm.io/gorm"

	"corefusion/internal/model/entity"
	"corefusion/internal/store/repository"
)

// ============================================================
// P1-2: Referral Service
// ============================================================

type ReferralService struct {
	db             *gorm.DB
	codeRepo       *repository.ReferralCodeRepository
	recordRepo     *repository.ReferralRecordRepository
	withdrawalRepo *repository.ReferralWithdrawalRepository
	balanceRepo    *repository.ClientBalanceRepository
	balanceTxRepo  *repository.BalanceTransactionRepository
	clientRepo     *repository.ClientRepository
	settingSvc     SettingStore
}

func NewReferralService(
	db *gorm.DB,
	codeRepo *repository.ReferralCodeRepository,
	recordRepo *repository.ReferralRecordRepository,
	withdrawalRepo *repository.ReferralWithdrawalRepository,
	balanceRepo *repository.ClientBalanceRepository,
	balanceTxRepo *repository.BalanceTransactionRepository,
	clientRepo *repository.ClientRepository,
	settingSvc SettingStore,
) *ReferralService {
	return &ReferralService{db, codeRepo, recordRepo, withdrawalRepo, balanceRepo, balanceTxRepo, clientRepo, settingSvc}
}

func (s *ReferralService) GetOrCreateCode(ctx context.Context, clientID int64) (*entity.ReferralCode, error) {
	code, err := s.codeRepo.GetByClientID(ctx, clientID)
	if err == nil {
		return code, nil
	}
	code = &entity.ReferralCode{ClientID: clientID, Code: generateReferralCode()}
	if err := s.codeRepo.Create(ctx, code); err != nil {
		return nil, err
	}
	return code, nil
}

type ReferralStats struct {
	TotalInvited        int64 `json:"total_invited"`
	TotalPaid          int64 `json:"total_paid"`
	TotalCommission    int64 `json:"total_commission"`
	AvailableCommission int64 `json:"available_commission"`
	PendingCommission  int64 `json:"pending_commission"`
}

func (s *ReferralService) GetStats(ctx context.Context, clientID int64) (*ReferralStats, error) {
	records, _, err := s.recordRepo.ListByReferrerID(ctx, clientID, 1000, 0)
	if err != nil {
		return nil, err
	}
	stats := &ReferralStats{}
	for _, r := range records {
		stats.TotalInvited++
		if r.FirstPaidAt != nil {
			stats.TotalPaid++
		}
		stats.TotalCommission += r.CommissionAmount
		if r.CommissionStatus == entity.CommissionAvailable {
			stats.AvailableCommission += r.CommissionAmount
		} else if r.CommissionStatus == entity.CommissionPending {
			stats.PendingCommission += r.CommissionAmount
		}
	}
	return stats, nil
}

func (s *ReferralService) ListRecords(ctx context.Context, clientID int64, limit, offset int) ([]entity.ReferralRecord, int64, error) {
	return s.recordRepo.ListByReferrerID(ctx, clientID, limit, offset)
}

func (s *ReferralService) ListWithdrawals(ctx context.Context, clientID int64, limit, offset int) ([]entity.ReferralWithdrawal, int64, error) {
	return s.withdrawalRepo.ListByClientID(ctx, clientID, limit, offset)
}

type WithdrawInput struct {
	Amount        int64  `json:"amount" binding:"required"`
	WithdrawType  string `json:"withdraw_type" binding:"required"`
	WalletAddress string `json:"wallet_address"`
}

func (s *ReferralService) RequestWithdrawal(ctx context.Context, clientID int64, input WithdrawInput) (*entity.ReferralWithdrawal, error) {
	stats, err := s.GetStats(ctx, clientID)
	if err != nil {
		return nil, err
	}
	minWithdraw := s.getMinWithdrawAmount()
	if input.Amount < minWithdraw {
		return nil, fmt.Errorf("minimum withdrawal amount is %d cents", minWithdraw)
	}
	if input.Amount > stats.AvailableCommission {
		return nil, fmt.Errorf("insufficient available commission")
	}
	wd := &entity.ReferralWithdrawal{
		ClientID: clientID, Amount: input.Amount,
		WithdrawType: input.WithdrawType, WalletAddress: input.WalletAddress,
		Status: entity.WithdrawalPending,
	}
	if err := s.withdrawalRepo.Create(ctx, wd); err != nil {
		return nil, err
	}
	return wd, nil
}

func (s *ReferralService) ProcessReferral(ctx context.Context, referrerCode string, newClientID int64) error {
	if referrerCode == "" {
		return nil
	}
	code, err := s.codeRepo.GetByCode(ctx, referrerCode)
	if err != nil || code.ClientID == newClientID {
		return nil
	}
	newClient, err := s.clientRepo.GetByID(ctx, newClientID)
	if err != nil {
		return err
	}
	record := &entity.ReferralRecord{
		ReferrerID: code.ClientID, RefereeID: newClientID,
		RefereeEmail: maskEmail(newClient.Email), RegisteredAt: time.Now(),
		CommissionStatus: entity.CommissionPending,
	}
	return s.recordRepo.Create(ctx, record)
}

func (s *ReferralService) ProcessFirstPaid(ctx context.Context, clientID int64, orderAmount int64) error {
	record, err := s.recordRepo.GetByRefereeID(ctx, clientID)
	if err != nil || record.FirstPaidAt != nil {
		return nil
	}
	rate := s.getCommissionRate()
	now := time.Now()
	record.FirstPaidAt = &now
	record.CommissionAmount = orderAmount * int64(rate) / 100
	record.CommissionStatus = entity.CommissionAvailable
	return s.recordRepo.Update(ctx, record)
}

func (s *ReferralService) AdminApproveWithdrawal(ctx context.Context, id int64, remark string) error {
	wd, err := s.withdrawalRepo.GetByID(ctx, id)
	if err != nil {
		return fmt.Errorf("withdrawal not found")
	}
	if wd.Status != entity.WithdrawalPending {
		return fmt.Errorf("withdrawal is not pending")
	}
	now := time.Now()
	wd.Status = entity.WithdrawalApproved
	wd.ProcessedAt = &now
	wd.ProcessRemark = remark
	if wd.WithdrawType == entity.WithdrawTypeBalance {
		if _, err := s.balanceRepo.AddBalance(ctx, wd.ClientID, wd.Amount); err != nil {
			return err
		}
		txRecord := &entity.BalanceTransaction{
			ClientID: wd.ClientID, Type: entity.BalanceTxTypeCharge,
			Amount: wd.Amount, Remark: "提现到余额",
		}
		s.balanceTxRepo.Create(ctx, txRecord)
	}
	records, _, _ := s.recordRepo.ListByReferrerID(ctx, wd.ClientID, 1000, 0)
	remaining := wd.Amount
	for _, r := range records {
		if r.CommissionStatus == entity.CommissionAvailable && remaining > 0 {
			deduct := r.CommissionAmount
			if deduct > remaining {
				deduct = remaining
			}
			remaining -= deduct
			r.CommissionStatus = entity.CommissionWithdrawn
			s.recordRepo.Update(ctx, &r)
		}
	}
	return s.withdrawalRepo.Update(ctx, wd)
}

func (s *ReferralService) AdminRejectWithdrawal(ctx context.Context, id int64, remark string) error {
	wd, err := s.withdrawalRepo.GetByID(ctx, id)
	if err != nil {
		return fmt.Errorf("withdrawal not found")
	}
	if wd.Status != entity.WithdrawalPending {
		return fmt.Errorf("withdrawal is not pending")
	}
	now := time.Now()
	wd.Status = entity.WithdrawalRejected
	wd.ProcessedAt = &now
	wd.ProcessRemark = remark
	return s.withdrawalRepo.Update(ctx, wd)
}

func (s *ReferralService) AdminListWithdrawals(ctx context.Context, status string, limit, offset int) ([]entity.ReferralWithdrawal, int64, error) {
	return s.withdrawalRepo.ListAll(ctx, status, limit, offset)
}

func (s *ReferralService) getCommissionRate() int {
	rate, _ := s.settingSvc.GetByKey(context.Background(), "referral.commission_rate")
	if rate == "" {
		return 15
	}
	r := 0
	fmt.Sscanf(rate, "%d", &r)
	if r <= 0 || r > 100 {
		return 15
	}
	return r
}

func (s *ReferralService) getMinWithdrawAmount() int64 {
	amt, _ := s.settingSvc.GetByKey(context.Background(), "referral.min_withdraw_amount")
	if amt == "" {
		return 1000
	}
	a := int64(0)
	fmt.Sscanf(amt, "%d", &a)
	if a <= 0 {
		return 1000
	}
	return a
}

func generateReferralCode() string {
	b := make([]byte, 4)
	rand.Read(b)
	return hex.EncodeToString(b)
}

// ============================================================
// P1-3: Ticket Service
// ============================================================

type TicketService struct {
	ticketRepo  *repository.TicketRepository
	messageRepo *repository.TicketMessageRepository
	clientRepo  *repository.ClientRepository
}

func NewTicketService(
	ticketRepo *repository.TicketRepository,
	messageRepo *repository.TicketMessageRepository,
	clientRepo *repository.ClientRepository,
) *TicketService {
	return &TicketService{ticketRepo, messageRepo, clientRepo}
}

type CreateTicketInput struct {
	Subject  string `json:"subject" binding:"required"`
	Category string `json:"category"`
	Priority string `json:"priority"`
	Content  string `json:"content" binding:"required"`
}

func (s *TicketService) CreateTicket(ctx context.Context, clientID int64, input CreateTicketInput) (*entity.Ticket, error) {
	if input.Category == "" {
		input.Category = entity.TicketCategoryOther
	}
	if input.Priority == "" {
		input.Priority = entity.TicketPriorityNormal
	}
	ticket := &entity.Ticket{
		ClientID: clientID, Subject: input.Subject,
		Category: input.Category, Priority: input.Priority,
		Status: entity.TicketStatusOpen,
	}
	if err := s.ticketRepo.Create(ctx, ticket); err != nil {
		return nil, err
	}
	msg := &entity.TicketMessage{
		TicketID: ticket.ID, SenderType: entity.SenderTypeClient,
		SenderID: clientID, Content: input.Content,
	}
	if err := s.messageRepo.Create(ctx, msg); err != nil {
		return nil, err
	}
	return ticket, nil
}

func (s *TicketService) ListTickets(ctx context.Context, clientID int64, status string, limit, offset int) ([]entity.Ticket, int64, error) {
	return s.ticketRepo.ListByClientID(ctx, clientID, status, limit, offset)
}

func (s *TicketService) GetTicket(ctx context.Context, clientID, ticketID int64) (*entity.Ticket, []entity.TicketMessage, error) {
	ticket, err := s.ticketRepo.GetByID(ctx, ticketID)
	if err != nil || ticket.ClientID != clientID {
		return nil, nil, fmt.Errorf("ticket not found")
	}
	msgs, _ := s.messageRepo.ListByTicketID(ctx, ticketID)
	return ticket, msgs, nil
}

func (s *TicketService) ReplyTicket(ctx context.Context, clientID, ticketID int64, content string) (*entity.TicketMessage, error) {
	ticket, err := s.ticketRepo.GetByID(ctx, ticketID)
	if err != nil || ticket.ClientID != clientID {
		return nil, fmt.Errorf("ticket not found")
	}
	if ticket.Status == entity.TicketStatusClosed {
		return nil, fmt.Errorf("ticket is closed")
	}
	msg := &entity.TicketMessage{
		TicketID: ticketID, SenderType: entity.SenderTypeClient,
		SenderID: clientID, Content: content,
	}
	if err := s.messageRepo.Create(ctx, msg); err != nil {
		return nil, err
	}
	ticket.Status = entity.TicketStatusOpen
	s.ticketRepo.Update(ctx, ticket)
	return msg, nil
}

func (s *TicketService) CloseTicket(ctx context.Context, clientID, ticketID int64) error {
	ticket, err := s.ticketRepo.GetByID(ctx, ticketID)
	if err != nil || ticket.ClientID != clientID {
		return fmt.Errorf("ticket not found")
	}
	ticket.Status = entity.TicketStatusClosed
	return s.ticketRepo.Update(ctx, ticket)
}

// Admin
func (s *TicketService) AdminListTickets(ctx context.Context, status, category string, limit, offset int) ([]entity.Ticket, int64, error) {
	return s.ticketRepo.ListAll(ctx, status, category, limit, offset)
}
func (s *TicketService) AdminGetTicket(ctx context.Context, ticketID int64) (*entity.Ticket, []entity.TicketMessage, error) {
	ticket, err := s.ticketRepo.GetByID(ctx, ticketID)
	if err != nil {
		return nil, nil, err
	}
	msgs, _ := s.messageRepo.ListByTicketID(ctx, ticketID)
	return ticket, msgs, nil
}
func (s *TicketService) AdminReplyTicket(ctx context.Context, ticketID, adminID int64, content string) error {
	ticket, err := s.ticketRepo.GetByID(ctx, ticketID)
	if err != nil {
		return fmt.Errorf("ticket not found")
	}
	msg := &entity.TicketMessage{
		TicketID: ticketID, SenderType: entity.SenderTypeAdmin,
		SenderID: adminID, Content: content,
	}
	if err := s.messageRepo.Create(ctx, msg); err != nil {
		return err
	}
	ticket.Status = entity.TicketStatusReplied
	if ticket.AssignedAdminID == nil {
		ticket.AssignedAdminID = &adminID
	}
	return s.ticketRepo.Update(ctx, ticket)
}
func (s *TicketService) AdminCloseTicket(ctx context.Context, ticketID int64) error {
	ticket, err := s.ticketRepo.GetByID(ctx, ticketID)
	if err != nil {
		return fmt.Errorf("ticket not found")
	}
	ticket.Status = entity.TicketStatusClosed
	return s.ticketRepo.Update(ctx, ticket)
}

// ============================================================
// P2-1: Gift Service
// ============================================================

type GiftService struct {
	giftRepo   *repository.ClientGiftRepository
	clientRepo *repository.ClientRepository
	subRepo    *repository.SubscriptionRepository
	db         *gorm.DB
}

func NewGiftService(giftRepo *repository.ClientGiftRepository, clientRepo *repository.ClientRepository, subRepo *repository.SubscriptionRepository, db *gorm.DB) *GiftService {
	return &GiftService{giftRepo, clientRepo, subRepo, db}
}

func (s *GiftService) ListAvailable(ctx context.Context, clientID int64) ([]entity.ClientGift, error) {
	return s.giftRepo.ListAvailableByClientID(ctx, clientID)
}

func (s *GiftService) ClaimGift(ctx context.Context, clientID, giftID int64) (*entity.ClientGift, error) {
	gift, err := s.giftRepo.GetByID(ctx, giftID)
	if err != nil || gift.ClientID != clientID {
		return nil, fmt.Errorf("gift not found")
	}
	if gift.Status != entity.GiftStatusAvailable {
		return nil, fmt.Errorf("gift is not available")
	}
	if gift.ExpiresAt != nil && gift.ExpiresAt.Before(time.Now()) {
		gift.Status = entity.GiftStatusExpired
		s.giftRepo.Update(ctx, gift)
		return nil, fmt.Errorf("gift has expired")
	}
	switch gift.Type {
	case entity.GiftTypeDays:
		client, err := s.clientRepo.GetByID(ctx, clientID)
		if err != nil {
			return nil, err
		}
		if client.ExpiresAt != nil {
			newExpiry := client.ExpiresAt.AddDate(0, 0, int(gift.Value))
			client.ExpiresAt = &newExpiry
			s.clientRepo.Update(ctx, client)
		}
	case entity.GiftTypeTraffic:
		client, err := s.clientRepo.GetByID(ctx, clientID)
		if err != nil {
			return nil, err
		}
		client.QuotaBytes += gift.Value
		s.clientRepo.Update(ctx, client)
		if client.SubscriptionID != nil {
			if sub, err := s.subRepo.GetByID(ctx, *client.SubscriptionID); err == nil {
				sub.QuotaBytes += gift.Value
				s.subRepo.Update(ctx, sub)
			}
		}
	}
	now := time.Now()
	gift.Status = entity.GiftStatusClaimed
	gift.ClaimedAt = &now
	if err := s.giftRepo.Update(ctx, gift); err != nil {
		return nil, err
	}
	return gift, nil
}

func (s *GiftService) AdminCreateGift(ctx context.Context, gift *entity.ClientGift) error {
	return s.giftRepo.Create(ctx, gift)
}
func (s *GiftService) AdminCreateGiftBatch(ctx context.Context, clientIDs []int64, giftType, source string, value int64, expiresAt *time.Time) error {
	gifts := make([]entity.ClientGift, len(clientIDs))
	for i, cid := range clientIDs {
		gifts[i] = entity.ClientGift{
			ClientID: cid, Type: giftType, Value: value,
			Source: source, Status: entity.GiftStatusAvailable, ExpiresAt: expiresAt,
		}
	}
	return s.giftRepo.CreateBatch(ctx, gifts)
}
func (s *GiftService) AdminListGifts(ctx context.Context, clientID int64, limit, offset int) ([]entity.ClientGift, int64, error) {
	return s.giftRepo.ListByClientID(ctx, clientID, limit, offset)
}

// ============================================================
// P2-2: Article Service
// ============================================================

type ArticleService struct {
	articleRepo *repository.ArticleRepository
}

func NewArticleService(articleRepo *repository.ArticleRepository) *ArticleService {
	return &ArticleService{articleRepo}
}

func (s *ArticleService) ListPublished(ctx context.Context, category, platform string) ([]entity.Article, error) {
	return s.articleRepo.ListPublished(ctx, category, platform)
}
func (s *ArticleService) GetBySlug(ctx context.Context, slug string) (*entity.Article, error) {
	return s.articleRepo.GetBySlug(ctx, slug)
}
func (s *ArticleService) ListAll(ctx context.Context, category string, limit, offset int) ([]entity.Article, int64, error) {
	return s.articleRepo.ListAll(ctx, category, limit, offset)
}
func (s *ArticleService) GetByID(ctx context.Context, id int64) (*entity.Article, error) {
	return s.articleRepo.GetByID(ctx, id)
}
func (s *ArticleService) Create(ctx context.Context, a *entity.Article) error {
	return s.articleRepo.Create(ctx, a)
}
func (s *ArticleService) Update(ctx context.Context, a *entity.Article) error {
	return s.articleRepo.Update(ctx, a)
}
func (s *ArticleService) Delete(ctx context.Context, id int64) error {
	return s.articleRepo.Delete(ctx, id)
}

// ============================================================
// P3-1: Shared Account Service
// ============================================================

type SharedAccountService struct {
	accountRepo   *repository.SharedAccountRepository
	repairLogRepo *repository.SharedAccountRepairLogRepository
	clientRepo    *repository.ClientRepository
}

func NewSharedAccountService(accountRepo *repository.SharedAccountRepository, repairLogRepo *repository.SharedAccountRepairLogRepository, clientRepo *repository.ClientRepository) *SharedAccountService {
	return &SharedAccountService{accountRepo, repairLogRepo, clientRepo}
}

func (s *SharedAccountService) ListVisible(ctx context.Context, clientID int64) ([]entity.SharedAccount, error) {
	client, err := s.clientRepo.GetByID(ctx, clientID)
	if err != nil {
		return nil, err
	}
	accounts, err := s.accountRepo.ListEnabled(ctx)
	if err != nil {
		return nil, err
	}
	clientCat := clientCategoryFromPlanID(client.PlanID)
	var visible []entity.SharedAccount
	for _, a := range accounts {
		if a.MinPlanCategory == "" || isCategorySufficient(clientCat, a.MinPlanCategory) {
			visible = append(visible, a)
		}
	}
	return visible, nil
}

func (s *SharedAccountService) GetAccount(ctx context.Context, clientID, accountID int64) (*entity.SharedAccount, error) {
	account, err := s.accountRepo.GetByID(ctx, accountID)
	if err != nil {
		return nil, fmt.Errorf("account not found")
	}
	client, err := s.clientRepo.GetByID(ctx, clientID)
	if err != nil {
		return nil, err
	}
	clientCat := clientCategoryFromPlanID(client.PlanID)
	if account.MinPlanCategory != "" && !isCategorySufficient(clientCat, account.MinPlanCategory) {
		return nil, fmt.Errorf("insufficient plan level")
	}
	return account, nil
}

func (s *SharedAccountService) RequestRepair(ctx context.Context, clientID, accountID int64) (*entity.SharedAccountRepairLog, error) {
	account, err := s.accountRepo.GetByID(ctx, accountID)
	if err != nil {
		return nil, fmt.Errorf("account not found")
	}
	lastLog, err := s.repairLogRepo.GetLastByClientAndAccount(ctx, clientID, accountID)
	if err == nil && lastLog != nil {
		cooldownMinutes := account.RepairCooldown
		if cooldownMinutes == 0 {
			cooldownMinutes = 1440
		}
		if time.Since(lastLog.CreatedAt) < time.Duration(cooldownMinutes)*time.Minute {
			return nil, fmt.Errorf("repair cooldown not expired")
		}
	}
	log := &entity.SharedAccountRepairLog{AccountID: accountID, ClientID: clientID, Status: entity.RepairStatusPending}
	if err := s.repairLogRepo.Create(ctx, log); err != nil {
		return nil, err
	}
	return log, nil
}

func (s *SharedAccountService) AdminList(ctx context.Context, limit, offset int) ([]entity.SharedAccount, int64, error) {
	return s.accountRepo.ListAll(ctx, limit, offset)
}
func (s *SharedAccountService) AdminGetAccount(ctx context.Context, id int64) (*entity.SharedAccount, error) {
	return s.accountRepo.GetByID(ctx, id)
}
func (s *SharedAccountService) AdminCreate(ctx context.Context, a *entity.SharedAccount) error {
	return s.accountRepo.Create(ctx, a)
}
func (s *SharedAccountService) AdminUpdate(ctx context.Context, a *entity.SharedAccount) error {
	return s.accountRepo.Update(ctx, a)
}
func (s *SharedAccountService) AdminDelete(ctx context.Context, id int64) error {
	return s.accountRepo.Delete(ctx, id)
}
func (s *SharedAccountService) AdminRepairLogs(ctx context.Context, accountID int64, limit, offset int) ([]entity.SharedAccountRepairLog, int64, error) {
	return s.repairLogRepo.ListByAccountID(ctx, accountID, limit, offset)
}

// clientCategoryFromPlanID derives a plan category from the client's plan ID.
func clientCategoryFromPlanID(planID int64) string {
	if planID <= 0 {
		return entity.PlanCategoryBasic
	}
	return entity.PlanCategoryStandard
}

func isCategorySufficient(clientCat, minCat string) bool {
	order := map[string]int{
		entity.PlanCategoryBasic: 1, entity.PlanCategoryStandard: 2,
		entity.PlanCategoryPremium: 3, entity.PlanCategoryTrial: 0, entity.PlanCategoryCustom: 4,
	}
	return order[clientCat] >= order[minCat]
}

// ============================================================
// P3-2: IP Whitelist Service
// ============================================================

type IPWhitelistService struct {
	ipRepo     *repository.IPWhitelistEntryRepository
	clientRepo *repository.ClientRepository
}

func NewIPWhitelistService(ipRepo *repository.IPWhitelistEntryRepository, clientRepo *repository.ClientRepository) *IPWhitelistService {
	return &IPWhitelistService{ipRepo, clientRepo}
}

func (s *IPWhitelistService) ListWhitelist(ctx context.Context, clientID int64) ([]entity.IPWhitelistEntry, error) {
	return s.ipRepo.ListByClientID(ctx, clientID)
}

func (s *IPWhitelistService) AutoAddIP(ctx context.Context, clientID int64, ip string) (*entity.IPWhitelistEntry, error) {
	existing, err := s.ipRepo.GetByClientAndIP(ctx, clientID, ip)
	if err == nil && existing != nil {
		return existing, nil
	}
	entry := &entity.IPWhitelistEntry{ClientID: clientID, IPAddress: ip, Source: entity.IPSourceAuto}
	if err := s.ipRepo.Create(ctx, entry); err != nil {
		return nil, err
	}
	client, err := s.clientRepo.GetByID(ctx, clientID)
	if err == nil {
		existingList := client.IPWhitelist
		if existingList != "" {
			existingList += ","
		}
		existingList += ip + ":auto"
		client.IPWhitelist = existingList
		s.clientRepo.Update(ctx, client)
	}
	return entry, nil
}

func (s *IPWhitelistService) RemoveIP(ctx context.Context, clientID, entryID int64) error {
	return s.ipRepo.DeleteByID(ctx, entryID)
}

// ============================================================
// P3-3: Self-Unban Service
// ============================================================

type SelfUnbanService struct {
	unbanLogRepo *repository.ClientSelfUnbanLogRepository
	clientRepo   *repository.ClientRepository
	settingSvc   SettingStore
}

func NewSelfUnbanService(unbanLogRepo *repository.ClientSelfUnbanLogRepository, clientRepo *repository.ClientRepository, settingSvc SettingStore) *SelfUnbanService {
	return &SelfUnbanService{unbanLogRepo, clientRepo, settingSvc}
}

func (s *SelfUnbanService) SelfUnban(ctx context.Context, clientID int64) error {
	client, err := s.clientRepo.GetByID(ctx, clientID)
	if err != nil {
		return fmt.Errorf("client not found")
	}
	if client.Status != "suspended" {
		return fmt.Errorf("account is not suspended")
	}
	cooldownHours := s.getCooldownHours()
	lastLog, err := s.unbanLogRepo.GetLastByClientID(ctx, clientID)
	if err == nil && lastLog != nil {
		if time.Since(lastLog.CreatedAt) < time.Duration(cooldownHours)*time.Hour {
			return fmt.Errorf("self-unban cooldown not expired (%d hours)", cooldownHours)
		}
	}
	client.Status = "active"
	if err := s.clientRepo.Update(ctx, client); err != nil {
		return err
	}
	log := &entity.ClientSelfUnbanLog{ClientID: clientID, Reason: "self_unban"}
	return s.unbanLogRepo.Create(ctx, log)
}

func (s *SelfUnbanService) getCooldownHours() int {
	val, _ := s.settingSvc.GetByKey(context.Background(), "self_unban.cooldown_hours")
	if val == "" {
		return 24
	}
	h := 0
	fmt.Sscanf(val, "%d", &h)
	if h <= 0 {
		return 24
	}
	return h
}

// ============================================================
// P2-3: Subscription Reset Service
// ============================================================

type SubscriptionResetService struct {
	subRepo    *repository.SubscriptionRepository
	clientRepo *repository.ClientRepository
}

func NewSubscriptionResetService(subRepo *repository.SubscriptionRepository, clientRepo *repository.ClientRepository) *SubscriptionResetService {
	return &SubscriptionResetService{subRepo, clientRepo}
}

func (s *SubscriptionResetService) ResetSubscription(ctx context.Context, clientID int64) error {
	client, err := s.clientRepo.GetByID(ctx, clientID)
	if err != nil {
		return fmt.Errorf("client not found")
	}
	if client.SubscriptionID == nil {
		return fmt.Errorf("no subscription found")
	}
	sub, err := s.subRepo.GetByID(ctx, *client.SubscriptionID)
	if err != nil {
		return err
	}
	sub.Token = generateToken(32)
	sub.TokenVersion++
	return s.subRepo.Update(ctx, sub)
}
