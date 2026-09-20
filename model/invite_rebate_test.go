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
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/setting/operation_setting"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestCalculateInviteRechargeRebateQuota(t *testing.T) {
	assert.Equal(t, 100_000, calculateInviteRechargeRebateQuota(1_000_000, 10))
	assert.Equal(t, 0, calculateInviteRechargeRebateQuota(1_000_000, 0))
	assert.Equal(t, 0, calculateInviteRechargeRebateQuota(0, 10))
	assert.Equal(t, 0, calculateInviteRechargeRebateQuota(1, 10))
	assert.Equal(t, 1_000_000, calculateInviteRechargeRebateQuota(1_000_000, 100))
	assert.Equal(t, 1_000_000, calculateInviteRechargeRebateQuota(1_000_000, 150))
	assert.Equal(t, 0, calculateInviteRechargeRebateQuota(1_000_000, -5))
}

func TestResolveInviterRebateRatePercent(t *testing.T) {
	old := common.QuotaRebateRateForInviter
	common.QuotaRebateRateForInviter = 12.5
	t.Cleanup(func() { common.QuotaRebateRateForInviter = old })

	assert.Equal(t, 12.5, ResolveInviterRebateRatePercent(nil))
	exclusive := 20.0
	assert.Equal(t, 20.0, ResolveInviterRebateRatePercent(&exclusive))
	zero := 0.0
	assert.Equal(t, 0.0, ResolveInviterRebateRatePercent(&zero))
}

func TestUserEditAppliesAndClearsExclusiveRebateRate(t *testing.T) {
	setupUserUpdateTestState(t)

	user := createUserBindTestUser(t)
	rate := 15.0
	edited := User{
		Id:                 user.Id,
		Username:           user.Username,
		DisplayName:        user.DisplayName,
		Group:              user.Group,
		AffRebateRate:      &rate,
		ApplyAffRebateRate: true,
	}
	require.NoError(t, edited.Edit(false))

	got, err := GetUserById(user.Id, false)
	require.NoError(t, err)
	require.NotNil(t, got.AffRebateRate)
	assert.Equal(t, 15.0, *got.AffRebateRate)

	cleared := User{
		Id:                 user.Id,
		Username:           user.Username,
		DisplayName:        user.DisplayName,
		Group:              user.Group,
		AffRebateRate:      nil,
		ApplyAffRebateRate: true,
	}
	require.NoError(t, cleared.Edit(false))

	got, err = GetUserById(user.Id, false)
	require.NoError(t, err)
	assert.Nil(t, got.AffRebateRate)

	rate = 8.0
	require.NoError(t, DB.Model(&User{}).Where("id = ?", user.Id).Update("aff_rebate_rate", rate).Error)
	untouched := User{
		Id:          user.Id,
		Username:    user.Username,
		DisplayName: "renamed",
		Group:       user.Group,
	}
	require.NoError(t, untouched.Edit(false))
	got, err = GetUserById(user.Id, false)
	require.NoError(t, err)
	require.NotNil(t, got.AffRebateRate)
	assert.Equal(t, 8.0, *got.AffRebateRate)
	assert.Equal(t, "renamed", got.DisplayName)
}

func TestRechargeEpayAccruesInviteRebateOnce(t *testing.T) {
	truncateTables(t)
	restoreInviteRebateTestState(t)

	oldQuotaPerUnit := common.QuotaPerUnit
	common.QuotaPerUnit = 500000
	t.Cleanup(func() { common.QuotaPerUnit = oldQuotaPerUnit })

	inviter := insertInviteRebateTestUser(t, 801, "rebate-inviter", "INV1", 0, 0)
	invitee := insertInviteRebateTestUser(t, 802, "rebate-invitee", "INV2", inviter.Id, 0)
	order := createEpayTestOrder(t, invitee.Id, "EPAYREBATEONCE", PaymentProviderEpay, common.TopUpStatusPending)

	alreadyDone, err := RechargeEpay(order.TradeNo, "alipay", "127.0.0.1")
	require.NoError(t, err)
	assert.False(t, alreadyDone)
	assert.Equal(t, 1_000_000, getUserQuotaForPaymentGuardTest(t, invitee.Id))
	assert.Equal(t, 100_000, getUserAffQuota(t, inviter.Id))
	assert.Equal(t, 100_000, getUserAffHistoryQuota(t, inviter.Id))

	alreadyDone, err = RechargeEpay(order.TradeNo, "alipay", "127.0.0.1")
	require.NoError(t, err)
	assert.True(t, alreadyDone)
	assert.Equal(t, 1_000_000, getUserQuotaForPaymentGuardTest(t, invitee.Id))
	assert.Equal(t, 100_000, getUserAffQuota(t, inviter.Id))
	assert.Equal(t, 100_000, getUserAffHistoryQuota(t, inviter.Id))
}

func TestRechargeEpayUsesExclusiveInviterRebateRate(t *testing.T) {
	truncateTables(t)
	restoreInviteRebateTestState(t)

	oldQuotaPerUnit := common.QuotaPerUnit
	common.QuotaPerUnit = 500000
	t.Cleanup(func() { common.QuotaPerUnit = oldQuotaPerUnit })

	exclusive := 20.0
	inviter := insertInviteRebateTestUser(t, 811, "rebate-inviter-ex", "INVX", 0, 0)
	require.NoError(t, DB.Model(inviter).Update("aff_rebate_rate", exclusive).Error)
	invitee := insertInviteRebateTestUser(t, 812, "rebate-invitee-ex", "INVY", inviter.Id, 0)
	order := createEpayTestOrder(t, invitee.Id, "EPAYREBATEEX", PaymentProviderEpay, common.TopUpStatusPending)

	_, err := RechargeEpay(order.TradeNo, "alipay", "127.0.0.1")
	require.NoError(t, err)
	assert.Equal(t, 200_000, getUserAffQuota(t, inviter.Id))
}

func TestRechargeEpaySkipsInviteRebateWhenExclusiveRateIsZero(t *testing.T) {
	truncateTables(t)
	restoreInviteRebateTestState(t)

	oldQuotaPerUnit := common.QuotaPerUnit
	common.QuotaPerUnit = 500000
	t.Cleanup(func() { common.QuotaPerUnit = oldQuotaPerUnit })

	zero := 0.0
	inviter := insertInviteRebateTestUser(t, 821, "rebate-inviter-zero", "INVZ", 0, 0)
	require.NoError(t, DB.Model(inviter).Update("aff_rebate_rate", zero).Error)
	invitee := insertInviteRebateTestUser(t, 822, "rebate-invitee-zero", "INVW", inviter.Id, 0)
	order := createEpayTestOrder(t, invitee.Id, "EPAYREBATEZERO", PaymentProviderEpay, common.TopUpStatusPending)

	_, err := RechargeEpay(order.TradeNo, "alipay", "127.0.0.1")
	require.NoError(t, err)
	assert.Equal(t, 1_000_000, getUserQuotaForPaymentGuardTest(t, invitee.Id))
	assert.Equal(t, 0, getUserAffQuota(t, inviter.Id))
}

func TestRechargeEpaySkipsInviteRebateWithoutInviter(t *testing.T) {
	truncateTables(t)
	restoreInviteRebateTestState(t)

	oldQuotaPerUnit := common.QuotaPerUnit
	common.QuotaPerUnit = 500000
	t.Cleanup(func() { common.QuotaPerUnit = oldQuotaPerUnit })

	invitee := insertInviteRebateTestUser(t, 831, "rebate-solo", "INVS", 0, 0)
	order := createEpayTestOrder(t, invitee.Id, "EPAYREBATESOLO", PaymentProviderEpay, common.TopUpStatusPending)

	_, err := RechargeEpay(order.TradeNo, "alipay", "127.0.0.1")
	require.NoError(t, err)
	assert.Equal(t, 1_000_000, getUserQuotaForPaymentGuardTest(t, invitee.Id))
	assert.Equal(t, 0, getUserAffQuota(t, invitee.Id))
}

func TestListInviteesReturnsInviteesAndTopupStats(t *testing.T) {
	truncateTables(t)
	restoreInviteRebateTestState(t)

	oldQuotaPerUnit := common.QuotaPerUnit
	common.QuotaPerUnit = 500000
	t.Cleanup(func() { common.QuotaPerUnit = oldQuotaPerUnit })

	exclusive := 15.0
	inviter := insertInviteRebateTestUser(t, 901, "list-inviter", "LIST", 0, 0)
	require.NoError(t, DB.Model(inviter).Update("aff_rebate_rate", exclusive).Error)
	first := insertInviteRebateTestUser(t, 902, "list-invitee-1", "LI1", inviter.Id, 42)
	second := insertInviteRebateTestUser(t, 903, "list-invitee-2", "LI2", inviter.Id, 7)
	_ = insertInviteRebateTestUser(t, 904, "list-unrelated", "LI3", 0, 0)

	order := createEpayTestOrder(t, first.Id, "EPAYLISTINV", PaymentProviderEpay, common.TopUpStatusPending)
	_, err := RechargeEpay(order.TradeNo, "alipay", "127.0.0.1")
	require.NoError(t, err)

	selfView, err := ListInvitees(inviter.Id, 0, 10, false)
	require.NoError(t, err)
	require.Len(t, selfView.Items, 2)
	assert.Equal(t, int64(2), selfView.Total)
	assert.Equal(t, exclusive, selfView.Inviter.EffectiveAffRebateRate)
	assert.Equal(t, RebateRateSourceExclusive, selfView.Inviter.RebateRateSource)
	assert.Equal(t, second.Id, selfView.Items[0].Id)
	assert.Equal(t, first.Id, selfView.Items[1].Id)
	assert.Nil(t, selfView.Items[0].Quota)
	assert.Equal(t, int64(1), selfView.Items[1].RechargeCount)
	assert.Equal(t, 10.0, selfView.Items[1].TotalPayMoney)
	assert.Greater(t, selfView.Items[1].LastRechargedAt, int64(0))
	assert.Equal(t, int64(0), selfView.Items[0].RechargeCount)

	adminView, err := ListInvitees(inviter.Id, 0, 10, true)
	require.NoError(t, err)
	require.NotNil(t, adminView.Items[0].Quota)
	assert.Equal(t, 7, *adminView.Items[0].Quota)
	require.NotNil(t, adminView.Items[1].Quota)
	assert.Equal(t, 1_000_042, *adminView.Items[1].Quota)
}

func restoreInviteRebateTestState(t *testing.T) {
	t.Helper()
	oldRate := common.QuotaRebateRateForInviter
	paymentSetting := operation_setting.GetPaymentSetting()
	oldConfirmed := paymentSetting.ComplianceConfirmed
	oldVersion := paymentSetting.ComplianceTermsVersion
	common.QuotaRebateRateForInviter = 10
	paymentSetting.ComplianceConfirmed = true
	paymentSetting.ComplianceTermsVersion = operation_setting.CurrentComplianceTermsVersion
	t.Cleanup(func() {
		common.QuotaRebateRateForInviter = oldRate
		paymentSetting.ComplianceConfirmed = oldConfirmed
		paymentSetting.ComplianceTermsVersion = oldVersion
	})
}

func insertInviteRebateTestUser(t *testing.T, id int, username string, affCode string, inviterId int, quota int) *User {
	t.Helper()
	user := &User{
		Id:        id,
		Username:  username,
		Status:    common.UserStatusEnabled,
		Quota:     quota,
		AffCode:   affCode,
		InviterId: inviterId,
	}
	require.NoError(t, DB.Create(user).Error)
	return user
}

func getUserAffQuota(t *testing.T, userId int) int {
	t.Helper()
	var user User
	require.NoError(t, DB.Select("aff_quota").First(&user, userId).Error)
	return user.AffQuota
}

func getUserAffHistoryQuota(t *testing.T, userId int) int {
	t.Helper()
	var user User
	require.NoError(t, DB.Select("aff_history").First(&user, userId).Error)
	return user.AffHistoryQuota
}
