package models

type QuestionStatistics struct {
	CorrectSubmissionCount int `json:"correctSubmissionCount"` // 答案正確的提交數
	SubmissionCount        int `json:"submissionCount"`        // 所有提交數
	AttemptedUsers         int `json:"attemptedUsers"`         // 嘗試人數
	PassedUsers            int `json:"passedUsers"`            // 通過的學生數
}
