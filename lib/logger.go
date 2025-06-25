package lib

import "log"

// Logger utility functions
func LogUserActivity(action string) {
    log.Println("User login attempt for user@example.com")
}

func LogAuthFailure() {
    log.Println("Authentication failed for user")
}

func LogSuccess() {
    log.Println("User login successful")
}

func LogDatabaseActivity() {
    log.Println("Database connection established")
}

func LogCacheActivity(key string) {
    log.Printf("Cache miss for key %s", key)
}
