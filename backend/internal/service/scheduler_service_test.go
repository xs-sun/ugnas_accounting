package service

import (
	"testing"
	"time"

	"com.xsguyu.accounting/backend/internal/model"
)

// 回归：定时规则以前只比日期，过了午夜就入账（2026-11-26 Justin 反馈：
// 8:28 打开 App，今天 9:00 的“支付宝的小荷包”¥2 已经记上了）。
// 现在 next_date == 今天的规则必须等到北京时间 9:00，逾期的才立即补记。
func TestShouldPostRule(t *testing.T) {
	loc := beijingLoc
	mk := func(nextDate string) model.RecurringRule {
		return model.RecurringRule{NextDate: nextDate}
	}
	at := func(hour, min int) time.Time {
		return time.Date(2026, 11, 26, hour, min, 0, 0, loc)
	}
	cases := []struct {
		name string
		rule model.RecurringRule
		now  time.Time
		want bool
	}{
		{"当天到期但未到9点不入账", mk("2026-11-26"), at(8, 28), false},
		{"当天到期午夜不入账", mk("2026-11-26"), at(0, 1), false},
		{"当天到期9点整入账", mk("2026-11-26"), at(9, 0), true},
		{"当天到期9点后入账", mk("2026-11-26"), at(9, 1), true},
		{"逾期规则9点前也补记", mk("2026-11-25"), at(8, 28), true},
		{"未到期不入账", mk("2026-11-27"), at(10, 0), false},
	}
	for _, c := range cases {
		if got := shouldPostRule(c.rule, c.now); got != c.want {
			t.Errorf("%s: shouldPostRule = %v, want %v", c.name, got, c.want)
		}
	}
}
