package main

import (
	"log"
	"net/http"
	"sort"
	"time"

	"github.com/labstack/echo/v4"
	"gorm.io/driver/mysql"
	"gorm.io/gorm"
)

type ActivityLog struct {
	UserID     int        `json:"user_id" gorm:"column:user_id"`
	ActivityID int        `json:"activity_id" gorm:"column:activity_id"`
	LoggedAt   *time.Time `json:"logged_at" gorm:"column:logged_at"`
}

// type ActivityLog struct {
// 	Id             uint       `json:"id" gorm:"index"`
// 	UserId         uint       `json:"user_id"`
// 	ActivityId     uint       `json:"activity_id"`
// 	Status         *uint      `json:"status"`
// 	ScheduledAt    *time.Time `json:"scheduled_at"`
// 	CreatedAt      *time.Time `json:"created_at,omitempty"`
// 	UpdatedAt      *time.Time `json:"updated_at,omitempty"`
// 	LastSetStatus  uint       `json:"last_set_status"` //to store last set status
// 	PreviousStatus *uint      `json:"previous_status"` //to store previous status after crossing habit threshold
// 	LoggedAt       *time.Time `json:"logged_at"`
// }

type Streak struct {
	UserID     int `json:"user_id"`
	Streak     int `json:"streak"`
	ActivityID int `json:"activity_id"`
}

type Points struct {
	UserID int `json:"user_id"`
	Points int `json:"points"`
}

var db *gorm.DB

func main() {
	e := echo.New()

	// Connecting database
	var err error
	dsn := "root:changedyou@tcp(localhost:3306)/wellteam?parseTime=true"
	db, err = gorm.Open(mysql.Open(dsn), &gorm.Config{})
	if err != nil {
		log.Fatalf("Could not connect to the database: %v", err)
	}
	// db.AutoMigrate(&ActivityLog{})
	// Routes
	e.GET("/streaks/:activity_id", getTopStreaks)
	e.GET("/points/:user_id", getPoints)

	// Start the server
	e.Logger.Fatal(e.Start(":8080"))
}

func getTopStreaks(c echo.Context) error {
	activityID := c.Param("activity_id")
	var activityLogs []ActivityLog

	err := db.Where("activity_id = ?", activityID).Order("user_id, logged_at").Find(&activityLogs).Error
	if err != nil {
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": err.Error()})
	}

	streaks := calculateStreaks(activityLogs)
	return c.JSON(http.StatusOK, streaks)
}

func calculateStreaks(activityLogs []ActivityLog) []Streak {
	userStreaks := make(map[int]int)
	currentStreaks := make(map[int]int)
	var prevUserID int
	var prevDate time.Time
	var activityID = activityLogs[0].ActivityID

	for _, activityLog := range activityLogs {
		if activityLog.LoggedAt == nil {
			continue
		}
		if activityLog.UserID != prevUserID {
			prevUserID = activityLog.UserID
			prevDate = *activityLog.LoggedAt
			currentStreaks[activityLog.UserID] = 1
			continue
		}

		if activityLog.LoggedAt.Sub(prevDate).Hours() <= 24 {
			currentStreaks[activityLog.UserID]++
		} else {
			if currentStreaks[activityLog.UserID] > userStreaks[activityLog.UserID] {
				userStreaks[activityLog.UserID] = currentStreaks[activityLog.UserID]
			}
			currentStreaks[activityLog.UserID] = 1
		}
		prevDate = *activityLog.LoggedAt
	}

	for userID, streak := range currentStreaks {
		if streak > userStreaks[userID] {
			userStreaks[userID] = streak
		}
	}

	streakList := []Streak{}
	for userID, streak := range userStreaks {
		streakList = append(streakList, Streak{UserID: userID, Streak: streak, ActivityID: activityID})
	}

	// Sort and return streakList
	sort.Slice(streakList, func(i, j int) bool {
		return streakList[i].Streak > streakList[j].Streak
	})
	// if len(streakList) > 3 {
	// 	streakList = streakList[:3]
	// }

	return streakList
}

func getPoints(c echo.Context) error {
	userID := c.Param("user_id")
	var points []Points

	// Query to join activities_v2 and activity_logs and calculate points
	err := db.Table("activity_logs").
		Select("activity_logs.user_id, SUM(activities_v2.points) as points").
		Joins("left join activities_v2 on activity_logs.activity_id = activities_v2.id").
		Where("activity_logs.user_id = ?", userID).
		Group("activity_logs.user_id").
		Scan(&points).Error
	if err != nil {
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": err.Error()})
	}

	if len(points) == 0 {
		return c.JSON(http.StatusOK, map[string]int{"points": 0})
	}
	return c.JSON(http.StatusOK, points[0])
}
