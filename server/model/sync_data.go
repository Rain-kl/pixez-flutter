package model

type BanComment struct {
	ID          uint   `gorm:"primaryKey;autoIncrement" json:"-"`
	PixivUserID string `gorm:"column:pixiv_user_id;not null;index" json:"-"`
	CommentID   string `gorm:"column:comment_id;not null" json:"comment_id"`
	Name        string `gorm:"column:name;not null" json:"name"`
}

func (BanComment) TableName() string { return "ban_comments" }

type BanIllust struct {
	ID          uint   `gorm:"primaryKey;autoIncrement" json:"-"`
	PixivUserID string `gorm:"column:pixiv_user_id;not null;index" json:"-"`
	IllustID    string `gorm:"column:illust_id;not null" json:"illust_id"`
	Name        string `gorm:"column:name;not null" json:"name"`
}

func (BanIllust) TableName() string { return "ban_illusts" }

type BanTag struct {
	ID            uint   `gorm:"primaryKey;autoIncrement" json:"-"`
	PixivUserID   string `gorm:"column:pixiv_user_id;not null;index" json:"-"`
	Name          string `gorm:"column:name;not null" json:"name"`
	TranslateName string `gorm:"column:translate_name;not null" json:"translate_name"`
}

func (BanTag) TableName() string { return "ban_tags" }

type BanUser struct {
	ID          uint   `gorm:"primaryKey;autoIncrement" json:"-"`
	PixivUserID string `gorm:"column:pixiv_user_id;not null;index" json:"-"`
	UserID      string `gorm:"column:user_id;not null" json:"user_id"`
	Name        string `gorm:"column:name;not null" json:"name"`
}

func (BanUser) TableName() string { return "ban_users" }

type IllustHistory struct {
	ID          uint   `gorm:"primaryKey;autoIncrement" json:"-"`
	PixivUserID string `gorm:"column:pixiv_user_id;not null;index" json:"-"`
	IllustID    int    `gorm:"column:illust_id;not null" json:"illust_id"`
	UserID      int    `gorm:"column:user_id;not null" json:"user_id"`
	PictureUrl  string `gorm:"column:picture_url;not null" json:"picture_url"`
	Title       string `gorm:"column:title" json:"title"`
	UserName    string `gorm:"column:user_name" json:"user_name"`
	Time        int64  `gorm:"column:time;not null" json:"time"`
}

func (IllustHistory) TableName() string { return "illust_histories" }

type NovelHistory struct {
	ID          uint   `gorm:"primaryKey;autoIncrement" json:"-"`
	PixivUserID string `gorm:"column:pixiv_user_id;not null;index" json:"-"`
	NovelID     int    `gorm:"column:novel_id;not null" json:"novel_id"`
	UserID      int    `gorm:"column:user_id;not null" json:"user_id"`
	PictureUrl  string `gorm:"column:picture_url;not null" json:"picture_url"`
	Title       string `gorm:"column:title;not null" json:"title"`
	UserName    string `gorm:"column:user_name;not null" json:"user_name"`
	Time        int64  `gorm:"column:time;not null" json:"time"`
}

func (NovelHistory) TableName() string { return "novel_histories" }

type TagHistory struct {
	ID             uint   `gorm:"primaryKey;autoIncrement" json:"-"`
	PixivUserID    string `gorm:"column:pixiv_user_id;not null;index" json:"-"`
	Name           string `gorm:"column:name;not null" json:"name"`
	TranslatedName string `gorm:"column:translated_name;not null" json:"translated_name"`
	Type           int    `gorm:"column:type" json:"type"`
}

func (TagHistory) TableName() string { return "tag_histories" }

// UserDataPayload holds the aggregated backup data for a specific user.
type UserDataPayload struct {
	BanComments     *[]BanComment    `json:"ban_comments,omitempty"`
	BanIllusts      *[]BanIllust     `json:"ban_illusts,omitempty"`
	BanTags         *[]BanTag        `json:"ban_tags,omitempty"`
	BanUsers        *[]BanUser       `json:"ban_users,omitempty"`
	IllustHistories *[]IllustHistory `json:"illust_histories,omitempty"`
	NovelHistories  *[]NovelHistory  `json:"novel_histories,omitempty"`
	TagHistories    *[]TagHistory    `json:"tag_histories,omitempty"`
}
