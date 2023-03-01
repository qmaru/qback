package common

const (
	MaxMsgSize  = 10 * 1024 * 1024
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
			  "InitialBackoff": "3s",
			  "MaxBackoff": "30s",
			  "BackoffMultiplier": 1.0,
			  "RetryableStatusCodes": [
				"UNAVAILABLE",
				"UNKNOWN"
			  ]
			}
		  }
		]
	  }`
)
