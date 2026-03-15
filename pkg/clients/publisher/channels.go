package publisher

import "fmt"

// UserChannel creates a channel name specifically for each individual user
func UserChannel(userID string) string {
	return fmt.Sprintf("client:%s", userID)
}

func MessageChannel(conversationID string) string {
	return fmt.Sprintf("conversation:%s", conversationID)
}
