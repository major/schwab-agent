package models

// UserPreference represents user preferences.
type UserPreference struct {
	Accounts     []UserPreferenceAccount `json:"accounts,omitempty"`
	Offers       []Offer                 `json:"offers,omitempty"`
	StreamerInfo []StreamerInfo          `json:"streamerInfo,omitempty"`
}

// UserPreferenceAccount represents an account in user preferences.
type UserPreferenceAccount struct {
	AccountNumber      *string `json:"accountNumber,omitempty"`
	PrimaryAccount     *bool   `json:"primaryAccount,omitempty"`
	Type               *string `json:"type,omitempty"`
	NickName           *string `json:"nickName,omitempty"`
	AccountColor       *string `json:"accountColor,omitempty"`
	DisplayAcctId      *string `json:"displayAcctId,omitempty"`
	AutoPositionEffect *bool   `json:"autoPositionEffect,omitempty"`
}

// Offer represents an offer in user preferences.
type Offer struct {
	ID          *string `json:"id,omitempty"`
	Name        *string `json:"name,omitempty"`
	Description *string `json:"description,omitempty"`
	Status      *string `json:"status,omitempty"`
}

// StreamerInfo represents streamer information in user preferences.
type StreamerInfo struct {
	StreamerURL  *string `json:"streamerUrl,omitempty"`
	Token        *string `json:"token,omitempty"`
	TokenExpTime *int64  `json:"tokenExpTime,omitempty"`
	AppID        *string `json:"appId,omitempty"`
	ACL          *string `json:"acl,omitempty"`
}
