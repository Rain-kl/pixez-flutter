package model

import "time"

// PixivUser represents the database schema for the Pixiv user credentials.
type PixivUser struct {
	PixivUserID      string    `gorm:"primaryKey;column:pixiv_user_id" json:"pixiv_user_id"`
	Name             string    `gorm:"not null" json:"name"`
	Account          string    `gorm:"not null" json:"account"`
	MailAddress      string    `json:"mail_address"`
	UserImage        string    `json:"user_image"`
	AccessToken      string    `gorm:"not null" json:"access_token"`
	RefreshToken     string    `gorm:"not null" json:"refresh_token"`
	DeviceToken      string    `json:"device_token"`
	IsPremium        int       `gorm:"default:0" json:"is_premium"`
	XRestrict        int       `gorm:"default:0" json:"x_restrict"`
	IsMailAuthorized int       `gorm:"default:0" json:"is_mail_authorized"`
	CreatedAt        time.Time `json:"created_at"`
	UpdatedAt        time.Time `json:"updated_at"`
}

// TableName overrides the table name for GORM.
func (PixivUser) TableName() string {
	return "pixiv_users"
}

// PixivUserSafeDTO is a safe representation of PixivUser, excluding sensitive tokens.
type PixivUserSafeDTO struct {
	PixivUserID      string    `json:"pixiv_user_id"`
	Name             string    `json:"name"`
	Account          string    `json:"account"`
	MailAddress      string    `json:"mail_address"`
	UserImage        string    `json:"user_image"`
	IsPremium        int       `json:"is_premium"`
	XRestrict        int       `json:"x_restrict"`
	IsMailAuthorized int       `json:"is_mail_authorized"`
	CreatedAt        time.Time `json:"created_at"`
	UpdatedAt        time.Time `json:"updated_at"`
}

// ToSafeDTO converts PixivUser model to its safe DTO.
func (u *PixivUser) ToSafeDTO() PixivUserSafeDTO {
	return PixivUserSafeDTO{
		PixivUserID:      u.PixivUserID,
		Name:             u.Name,
		Account:          u.Account,
		MailAddress:      u.MailAddress,
		UserImage:        u.UserImage,
		IsPremium:        u.IsPremium,
		XRestrict:        u.XRestrict,
		IsMailAuthorized: u.IsMailAuthorized,
		CreatedAt:        u.CreatedAt,
		UpdatedAt:        u.UpdatedAt,
	}
}
