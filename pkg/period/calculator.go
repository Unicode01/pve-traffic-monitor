package period

import (
	"fmt"
	"time"
)

// PeriodType 周期类型
type PeriodType string

const (
	PeriodHour  PeriodType = "hour"
	PeriodDay   PeriodType = "day"
	PeriodMonth PeriodType = "month"
)

// Calculator 周期计算器
type Calculator struct {
	periodType      PeriodType
	creationTime    time.Time
	useCreationTime bool // 是否使用创建时间作为周期基准
}

// NewCalculator 创建周期计算器
func NewCalculator(periodType string, creationTime time.Time, useCreationTime bool) *Calculator {
	return &Calculator{
		periodType:      PeriodType(periodType),
		creationTime:    creationTime,
		useCreationTime: useCreationTime,
	}
}

// GetCurrentPeriodStart 获取当前周期开始时间
func (c *Calculator) GetCurrentPeriodStart() time.Time {
	now := time.Now()

	if !c.useCreationTime || c.creationTime.IsZero() {
		// 使用固定周期（月初/日初/小时初）
		return c.getFixedPeriodStart(now)
	}

	// 使用创建时间作为基准
	return c.getCreationBasedPeriodStart(now)
}

// GetNextPeriodStart 获取下一个周期开始时间
func (c *Calculator) GetNextPeriodStart() time.Time {
	currentStart := c.GetCurrentPeriodStart()

	if c.useCreationTime && !c.creationTime.IsZero() {
		return CalculateNextCreationBasedPeriodStart(string(c.periodType), c.creationTime, time.Now())
	}

	switch c.periodType {
	case PeriodHour:
		return currentStart.Add(1 * time.Hour)
	case PeriodDay:
		return currentStart.AddDate(0, 0, 1)
	case PeriodMonth:
		return currentStart.AddDate(0, 1, 0)
	default:
		return currentStart.Add(1 * time.Hour)
	}
}

// GetPeriodRange 获取当前周期的时间范围
func (c *Calculator) GetPeriodRange() (start time.Time, end time.Time) {
	start = c.GetCurrentPeriodStart()
	end = c.GetNextPeriodStart()
	return
}

// getFixedPeriodStart 获取固定周期的开始时间（传统方式）
func (c *Calculator) getFixedPeriodStart(now time.Time) time.Time {
	switch c.periodType {
	case PeriodHour:
		return time.Date(now.Year(), now.Month(), now.Day(), now.Hour(), 0, 0, 0, now.Location())
	case PeriodDay:
		return time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location())
	case PeriodMonth:
		return time.Date(now.Year(), now.Month(), 1, 0, 0, 0, 0, now.Location())
	default:
		return time.Date(now.Year(), now.Month(), now.Day(), now.Hour(), 0, 0, 0, now.Location())
	}
}

// getCreationBasedPeriodStart 基于创建时间计算周期开始时间
func (c *Calculator) getCreationBasedPeriodStart(now time.Time) time.Time {
	return CalculateCreationBasedPeriodStart(string(c.periodType), c.creationTime, now)
}

// CalculateCreationBasedPeriodStart 基于创建时间计算当前周期开始时间。
func CalculateCreationBasedPeriodStart(periodType string, creation, now time.Time) time.Time {
	if creation.IsZero() {
		return now
	}

	switch PeriodType(periodType) {
	case PeriodHour:
		// 从创建时间开始，每小时一个周期
		hoursSinceCreation := int(now.Sub(creation).Hours())
		return creation.Add(time.Duration(hoursSinceCreation) * time.Hour)

	case PeriodDay:
		// 从创建时间开始，每天一个周期
		// 保持创建时的小时和分钟
		daysSinceCreation := int(now.Sub(creation).Hours() / 24)
		return creation.AddDate(0, 0, daysSinceCreation)

	case PeriodMonth:
		// 从创建时间开始，每月一个周期
		// 例如：创建于 1月15日，则每月15日为周期开始
		// 计算从创建到现在经过了多少个月
		return monthPeriodStart(creation, monthPeriodIndex(creation, now))

	default:
		return now
	}
}

// CalculateNextCreationBasedPeriodStart 基于创建时间计算下一个周期开始时间。
func CalculateNextCreationBasedPeriodStart(periodType string, creation, now time.Time) time.Time {
	if creation.IsZero() {
		return now
	}

	currentStart := CalculateCreationBasedPeriodStart(periodType, creation, now)
	switch PeriodType(periodType) {
	case PeriodHour:
		return currentStart.Add(1 * time.Hour)
	case PeriodDay:
		return currentStart.AddDate(0, 0, 1)
	case PeriodMonth:
		return monthPeriodStart(creation, monthPeriodIndex(creation, now)+1)
	default:
		return now
	}
}

func monthPeriodIndex(creation, now time.Time) int {
	monthsSinceCreation := (now.Year()-creation.Year())*12 + int(now.Month()-creation.Month())
	candidate := monthPeriodStart(creation, monthsSinceCreation)
	if now.Before(candidate) {
		monthsSinceCreation--
	}
	return monthsSinceCreation
}

func monthPeriodStart(creation time.Time, monthsSinceCreation int) time.Time {
	year, month := addMonths(creation.Year(), creation.Month(), monthsSinceCreation)
	day := minInt(creation.Day(), daysInMonth(year, month))
	return time.Date(year, month, day,
		creation.Hour(), creation.Minute(), 0, 0, creation.Location())
}

func addMonths(year int, month time.Month, delta int) (int, time.Month) {
	monthIndex := int(month) - 1 + delta
	year += monthIndex / 12
	monthIndex = monthIndex % 12
	if monthIndex < 0 {
		monthIndex += 12
		year--
	}
	return year, time.Month(monthIndex + 1)
}

func daysInMonth(year int, month time.Month) int {
	return time.Date(year, month+1, 0, 0, 0, 0, 0, time.Local).Day()
}

func minInt(a, b int) int {
	if a < b {
		return a
	}
	return b
}

// FormatPeriod 格式化周期描述
func (c *Calculator) FormatPeriod() string {
	start, end := c.GetPeriodRange()

	periodName := ""
	switch c.periodType {
	case PeriodHour:
		periodName = "小时"
	case PeriodDay:
		periodName = "每日"
	case PeriodMonth:
		periodName = "每月"
	}

	if c.useCreationTime && !c.creationTime.IsZero() {
		return fmt.Sprintf("%s周期 (基于创建时间: %s - %s)",
			periodName,
			start.Format("01-02 15:04"),
			end.Format("01-02 15:04"))
	}

	return fmt.Sprintf("%s周期 (固定周期: %s - %s)",
		periodName,
		start.Format("01-02 15:04"),
		end.Format("01-02 15:04"))
}

// IsInCurrentPeriod 检查给定时间是否在当前周期内
func (c *Calculator) IsInCurrentPeriod(t time.Time) bool {
	start, end := c.GetPeriodRange()
	return (t.After(start) || t.Equal(start)) && t.Before(end)
}

// GetPeriodProgress 获取当前周期进度（0.0 - 1.0）
func (c *Calculator) GetPeriodProgress() float64 {
	start, end := c.GetPeriodRange()
	now := time.Now()

	if now.Before(start) {
		return 0.0
	}
	if now.After(end) {
		return 1.0
	}

	total := end.Sub(start).Seconds()
	elapsed := now.Sub(start).Seconds()

	return elapsed / total
}
