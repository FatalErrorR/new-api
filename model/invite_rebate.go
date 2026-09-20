/*
Copyright (C) 2023-2026 QuantumNous

This program is free software: you can redistribute it and/or modify
it under the terms of the GNU Affero General Public License as
published by the Free Software Foundation, either version 3 of the
License, or (at your option) any later version.

This program is distributed in the hope that it will be useful,
but WITHOUT ANY WARRANTY; without even the implied warranty of
MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE. See the
GNU Affero General Public License for more details.

You should have received a copy of the GNU Affero General Public License
along with this program. If not, see <https://www.gnu.org/licenses/>.

For commercial licensing, please contact support@quantumnous.com
*/
package model

import (
	"errors"
	"fmt"
	"math"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/logger"
	"github.com/QuantumNous/new-api/setting/operation_setting"

	"github.com/shopspring/decimal"
	"gorm.io/gorm"
)

const (
	inviteRebateRateMin = 0.0
	inviteRebateRateMax = 100.0
)

type inviteRechargeRebate struct {
	InviterId int
	Amount    int
}

func ValidateAffRebateRate(rate *float64) error {
	if rate == nil {
		return nil
	}
	if math.IsNaN(*rate) || math.IsInf(*rate, 0) || *rate < inviteRebateRateMin || *rate > inviteRebateRateMax {
		return errors.New("aff_rebate_rate must be between 0 and 100")
	}
	return nil
}

func ClampRebateRatePercent(rate float64) float64 {
	if math.IsNaN(rate) || math.IsInf(rate, 0) || rate < inviteRebateRateMin {
		return inviteRebateRateMin
	}
	if rate > inviteRebateRateMax {
		return inviteRebateRateMax
	}
	return rate
}

func ResolveInviterRebateRatePercent(exclusive *float64) float64 {
	if exclusive != nil {
		return ClampRebateRatePercent(*exclusive)
	}
	return ClampRebateRatePercent(common.QuotaRebateRateForInviter)
}

func calculateInviteRechargeRebateQuota(creditedQuota int, ratePercent float64) int {
	ratePercent = ClampRebateRatePercent(ratePercent)
	if creditedQuota <= 0 || ratePercent <= 0 {
		return 0
	}
	rebateDec := decimal.NewFromInt(int64(creditedQuota)).
		Mul(decimal.NewFromFloat(ratePercent)).
		Div(decimal.NewFromInt(100))
	rebate, err := common.QuotaFromDecimalStrict(rebateDec)
	if err != nil || rebate <= 0 {
		return 0
	}
	return rebate
}

func creditTopUpQuotaWithInviteRebate(tx *gorm.DB, userId int, creditedQuota int, updates map[string]interface{}) (inviteRechargeRebate, error) {
	if err := creditTopUpQuota(tx, userId, creditedQuota, updates); err != nil {
		return inviteRechargeRebate{}, err
	}
	return accrueInviteRechargeRebate(tx, userId, creditedQuota)
}

func accrueInviteRechargeRebate(tx *gorm.DB, inviteeUserId int, creditedQuota int) (inviteRechargeRebate, error) {
	if tx == nil || inviteeUserId <= 0 || creditedQuota <= 0 {
		return inviteRechargeRebate{}, nil
	}
	if !operation_setting.IsPaymentComplianceConfirmed() {
		return inviteRechargeRebate{}, nil
	}

	var invitee User
	if err := tx.Select("id", "inviter_id").First(&invitee, inviteeUserId).Error; err != nil {
		return inviteRechargeRebate{}, err
	}
	if invitee.InviterId <= 0 || invitee.InviterId == inviteeUserId {
		return inviteRechargeRebate{}, nil
	}

	var inviter User
	if err := tx.Select("id", "aff_rebate_rate").First(&inviter, invitee.InviterId).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return inviteRechargeRebate{}, nil
		}
		return inviteRechargeRebate{}, err
	}

	rebate := calculateInviteRechargeRebateQuota(creditedQuota, ResolveInviterRebateRatePercent(inviter.AffRebateRate))
	if rebate <= 0 {
		return inviteRechargeRebate{}, nil
	}

	result := tx.Model(&User{}).Where("id = ?", inviter.Id).Updates(map[string]interface{}{
		"aff_quota":   gorm.Expr("aff_quota + ?", rebate),
		"aff_history": gorm.Expr("aff_history + ?", rebate),
	})
	if result.Error != nil {
		return inviteRechargeRebate{}, result.Error
	}
	if result.RowsAffected == 0 {
		return inviteRechargeRebate{}, nil
	}
	return inviteRechargeRebate{InviterId: inviter.Id, Amount: rebate}, nil
}

func recordInviteRechargeRebateLog(inviteeId int, creditedQuota int, rebate inviteRechargeRebate) {
	if rebate.Amount <= 0 || rebate.InviterId <= 0 {
		return
	}
	RecordLog(rebate.InviterId, LogTypeSystem, fmt.Sprintf(
		"邀请充值返利 %s（来源用户 %d，充值 %s）",
		logger.LogQuota(rebate.Amount),
		inviteeId,
		logger.LogQuota(creditedQuota),
	))
}

const (
	RebateRateSourceExclusive = "exclusive"
	RebateRateSourceGlobal    = "global"
)

type InviteeListInviter struct {
	Id                     int      `json:"id"`
	Username               string   `json:"username"`
	DisplayName            string   `json:"display_name"`
	AffCode                string   `json:"aff_code"`
	AffCount               int      `json:"aff_count"`
	AffQuota               int      `json:"aff_quota"`
	AffHistoryQuota        int      `json:"aff_history_quota"`
	AffRebateRate          *float64 `json:"aff_rebate_rate"`
	EffectiveAffRebateRate float64  `json:"effective_aff_rebate_rate"`
	RebateRateSource       string   `json:"rebate_rate_source"`
}

type InviteeListItem struct {
	Id              int     `json:"id"`
	Username        string  `json:"username"`
	DisplayName     string  `json:"display_name"`
	Status          int     `json:"status"`
	CreatedAt       int64   `json:"created_at"`
	LastLoginAt     int64   `json:"last_login_at"`
	RechargeCount   int64   `json:"recharge_count"`
	TotalPayMoney   float64 `json:"total_pay_money"`
	LastRechargedAt int64   `json:"last_recharged_at"`
	Quota           *int    `json:"quota,omitempty"`
	Group           string  `json:"group,omitempty"`
}

type InviteeListResult struct {
	Inviter  InviteeListInviter `json:"inviter"`
	Items    []InviteeListItem  `json:"items"`
	Total    int64              `json:"total"`
	Page     int                `json:"page"`
	PageSize int                `json:"page_size"`
}

type inviteeTopUpStat struct {
	UserId          int     `gorm:"column:user_id"`
	RechargeCount   int64   `gorm:"column:recharge_count"`
	TotalPayMoney   float64 `gorm:"column:total_pay_money"`
	LastRechargedAt int64   `gorm:"column:last_recharged_at"`
}

func ListInvitees(inviterId int, startIdx int, num int, includeAdminFields bool) (*InviteeListResult, error) {
	if inviterId <= 0 {
		return nil, errors.New("inviter id is empty")
	}
	inviter, err := GetUserById(inviterId, false)
	if err != nil {
		return nil, err
	}
	if num <= 0 {
		num = common.ItemsPerPage
	}
	if startIdx < 0 {
		startIdx = 0
	}

	result := &InviteeListResult{
		Inviter: InviteeListInviter{
			Id:                     inviter.Id,
			Username:               inviter.Username,
			DisplayName:            inviter.DisplayName,
			AffCode:                inviter.AffCode,
			AffCount:               inviter.AffCount,
			AffQuota:               inviter.AffQuota,
			AffHistoryQuota:        inviter.AffHistoryQuota,
			AffRebateRate:          inviter.AffRebateRate,
			EffectiveAffRebateRate: ResolveInviterRebateRatePercent(inviter.AffRebateRate),
			RebateRateSource:       RebateRateSourceGlobal,
		},
		Items: make([]InviteeListItem, 0),
	}
	if inviter.AffRebateRate != nil {
		result.Inviter.RebateRateSource = RebateRateSourceExclusive
	}

	query := DB.Model(&User{}).Where("inviter_id = ?", inviterId)
	if err := query.Count(&result.Total).Error; err != nil {
		return nil, err
	}
	if result.Total == 0 {
		return result, nil
	}

	var users []User
	if err := DB.Where("inviter_id = ?", inviterId).
		Order("id desc").
		Limit(num).
		Offset(startIdx).
		Omit("password", "access_token").
		Find(&users).Error; err != nil {
		return nil, err
	}

	ids := make([]int, 0, len(users))
	for _, user := range users {
		ids = append(ids, user.Id)
	}
	stats := loadInviteeTopUpStats(ids)

	for _, user := range users {
		item := InviteeListItem{
			Id:          user.Id,
			Username:    user.Username,
			DisplayName: user.DisplayName,
			Status:      user.Status,
			CreatedAt:   user.CreatedAt,
			LastLoginAt: user.LastLoginAt,
		}
		if stat, ok := stats[user.Id]; ok {
			item.RechargeCount = stat.RechargeCount
			item.TotalPayMoney = stat.TotalPayMoney
			item.LastRechargedAt = stat.LastRechargedAt
		}
		if includeAdminFields {
			quota := user.Quota
			item.Quota = &quota
			item.Group = user.Group
		}
		result.Items = append(result.Items, item)
	}
	return result, nil
}

func loadInviteeTopUpStats(userIds []int) map[int]inviteeTopUpStat {
	stats := make(map[int]inviteeTopUpStat, len(userIds))
	if len(userIds) == 0 {
		return stats
	}
	var rows []inviteeTopUpStat
	err := DB.Model(&TopUp{}).
		Select("user_id, COUNT(*) AS recharge_count, COALESCE(SUM(money), 0) AS total_pay_money, COALESCE(MAX(complete_time), 0) AS last_recharged_at").
		Where("user_id IN ? AND status = ?", userIds, common.TopUpStatusSuccess).
		Group("user_id").
		Scan(&rows).Error
	if err != nil {
		common.SysError("load invitee topup stats failed: " + err.Error())
		return stats
	}
	for _, row := range rows {
		stats[row.UserId] = row
	}
	return stats
}
