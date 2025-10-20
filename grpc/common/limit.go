package common

const (
	MaxMsgSize  = 50 * 1024 * 1024
	RetryPolicy = `{
		"methodConfig": [
		  {
			"name": [
			  {
				"service": "pb.FileTransferService"
			  }
			],
			"retryPolicy": {
			  "MaxAttempts": 4,
			  "InitialBackoff": "1s",
			  "MaxBackoff": "30s",
			  "BackoffMultiplier": 2.0,
			  "RetryableStatusCodes": [
				"UNAVAILABLE",
				"UNKNOWN",
				"DEADLINE_EXCEEDED"
			  ]
			}
		  }
		]
	  }`
)
