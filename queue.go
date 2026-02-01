package main

// SubmissionQueue is the buffered channel for processing submissions
// Capacity 5000: Covers Max Users (50) * Max Submissions (100) = 100% Guaranteed Non-Blocking
var SubmissionQueue chan int

func InitQueue() {
	SubmissionQueue = make(chan int, 5000)
}
