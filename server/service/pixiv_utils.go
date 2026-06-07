package service

import (
	"bytes"
	"crypto/md5"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/url"
	"os"
	"regexp"
	"strings"
	"time"

	"pixez-sync/db"
	"pixez-sync/model"
)

const (
	pixivAppUserAgent = "PixivAndroidApp/5.0.166 (Android 16; PKX110)"
	pixivAppVersion   = "5.0.166"
	pixivAppOS        = "Android"
	pixivAppOSVersion = "Android 16"
	pixivAPIHost      = "app-api.pixiv.net"
)

var pixivClient = &http.Client{Timeout: 30 * time.Second}

// PixivUtils centralizes all official Pixiv API requests and response parsing.
type PixivUtils struct {
	Client *http.Client
}

func NewPixivUtils() *PixivUtils {
	return &PixivUtils{Client: pixivClient}
}

type PixivIllustDetail struct {
	Illust PixivIllust `json:"illust"`
}

type PixivBookmarkIllustResponse struct {
	Illusts []PixivIllust `json:"illusts"`
	NextURL string        `json:"next_url"`
}

type PixivIllust struct {
	ID        int64  `json:"id"`
	Title     string `json:"title"`
	Type      string `json:"type"`
	ImageUrls struct {
		SquareMedium string `json:"square_medium"`
		Medium       string `json:"medium"`
		Large        string `json:"large"`
	} `json:"image_urls"`
	Caption  string `json:"caption"`
	Restrict int    `json:"restrict"`
	User     struct {
		ID               int64  `json:"id"`
		Name             string `json:"name"`
		Account          string `json:"account"`
		ProfileImageUrls struct {
			Medium string `json:"medium"`
		} `json:"profile_image_urls"`
		IsFollowed      bool `json:"is_followed"`
		IsAcceptRequest bool `json:"is_accept_request"`
	} `json:"user"`
	Tags []struct {
		Name           string  `json:"name"`
		TranslatedName *string `json:"translated_name"`
	} `json:"tags"`
	Tools          []string `json:"tools"`
	CreateDate     string   `json:"create_date"`
	PageCount      int      `json:"page_count"`
	Width          int      `json:"width"`
	Height         int      `json:"height"`
	SanityLevel    int      `json:"sanity_level"`
	XRestrict      int      `json:"x_restrict"`
	Series         any      `json:"series"`
	MetaSinglePage struct {
		OriginalImageURL string `json:"original_image_url"`
	} `json:"meta_single_page"`
	MetaPages []struct {
		ImageUrls struct {
			SquareMedium string `json:"square_medium"`
			Medium       string `json:"medium"`
			Large        string `json:"large"`
			Original     string `json:"original"`
		} `json:"image_urls"`
	} `json:"meta_pages"`
	TotalView             int      `json:"total_view"`
	TotalBookmarks        int      `json:"total_bookmarks"`
	IsBookmarked          bool     `json:"is_bookmarked"`
	Visible               bool     `json:"visible"`
	IsMuted               bool     `json:"is_muted"`
	IllustAIType          int      `json:"illust_ai_type"`
	IllustBookStyle       int      `json:"illust_book_style"`
	Request               any      `json:"request"`
	RestrictionAttributes []string `json:"restriction_attributes"`
}

func (u *PixivUtils) GetIllustDetail(user model.PixivUser, illustID int64) ([]byte, PixivIllustDetail, error) {
	reqURL := fmt.Sprintf("https://%s/v1/illust/detail?filter=for_android&illust_id=%d", pixivAPIHost, illustID)
	var detail PixivIllustDetail
	data, err := u.GetJSONWithAuth(user, reqURL, &detail)
	if err != nil {
		return nil, detail, err
	}
	if detail.Illust.ID == 0 {
		return nil, detail, fmt.Errorf("Pixiv response for illust_id=%d does not contain illust.id", illustID)
	}
	return data, detail, nil
}

// PixivNovelDetail mirrors the Pixiv /v2/novel/detail response shape.
type PixivNovelDetail struct {
	Novel PixivNovel `json:"novel"`
}

type PixivNovel struct {
	ID    int64  `json:"id"`
	Title string `json:"title"`
	User  struct {
		ID               int64  `json:"id"`
		Name             string `json:"name"`
		Account          string `json:"account"`
		ProfileImageUrls struct {
			Medium string `json:"medium"`
		} `json:"profile_image_urls"`
		IsFollowed bool `json:"is_followed"`
	} `json:"user"`
	Caption    string `json:"caption"`
	CreateDate string `json:"create_date"`
	Tags       []struct {
		Name           string  `json:"name"`
		TranslatedName *string `json:"translated_name"`
	} `json:"tags"`
	PageCount      int  `json:"page_count"`
	TextLength     int  `json:"text_length"`
	TotalBookmarks int  `json:"total_bookmarks"`
	TotalView      int  `json:"total_view"`
	IsBookmarked   bool `json:"is_bookmarked"`
	IsMuted        bool `json:"is_muted"`
	NovelAIType    int  `json:"novel_ai_type"`
	ImageUrls      struct {
		SquareMedium string `json:"square_medium"`
		Medium       string `json:"medium"`
		Large        string `json:"large"`
	} `json:"image_urls"`
	Series struct {
		ID    *int64  `json:"id"`
		Title *string `json:"title"`
	} `json:"series"`
}

// PixivNovelWebContent holds the novel JSON extracted from the /webview/v2/novel HTML response.
// The HTML embeds novel data inside a <script> tag as: novel: {...},\n  isOwnWork
type PixivNovelWebContent struct {
	Text string `json:"text"`
}

// GetNovelText fetches novel text content via /webview/v2/novel and parses the embedded JSON.
// This mirrors PixEz's novel_store.dart _parseHtml logic which extracts novel JSON from a <script> tag.
func (u *PixivUtils) GetNovelText(user model.PixivUser, novelID int64) ([]byte, PixivNovelWebContent, error) {
	reqURL := fmt.Sprintf("https://%s/webview/v2/novel?id=%d", pixivAPIHost, novelID)
	data, status, err := u.doPixivAPIGet(reqURL, user.AccessToken)
	if err != nil {
		return nil, PixivNovelWebContent{}, err
	}
	if status != http.StatusOK {
		return nil, PixivNovelWebContent{}, fmt.Errorf("request=%s status=%d response=%s", reqURL, status, string(data))
	}

	// Extract novel JSON from the HTML <script> block, same as PixEz's _parseHtml()
	// Pattern: novel: ({...}),\n  isOwnWork
	novelRegex := regexp.MustCompile(`(?s)novel:\s*(\{.*?\}),\s*\n\s*isOwnWork`)
	matches := novelRegex.FindSubmatch(data)
	if matches == nil || len(matches) < 2 {
		return nil, PixivNovelWebContent{}, fmt.Errorf("novel JSON not found in webview response for novel_id=%d", novelID)
	}
	novelJSONBytes := matches[1]

	var content PixivNovelWebContent
	if err := json.Unmarshal(novelJSONBytes, &content); err != nil {
		return nil, PixivNovelWebContent{}, fmt.Errorf("parse novel webview JSON failed: %w", err)
	}
	return novelJSONBytes, content, nil
}

// GetNovelDetail fetches novel detail from Pixiv /v2/novel/detail.
func (u *PixivUtils) GetNovelDetail(user model.PixivUser, novelID int64) ([]byte, PixivNovelDetail, error) {
	reqURL := fmt.Sprintf("https://%s/v2/novel/detail?novel_id=%d", pixivAPIHost, novelID)
	var detail PixivNovelDetail
	data, err := u.GetJSONWithAuth(user, reqURL, &detail)
	if err != nil {
		return nil, detail, err
	}
	if detail.Novel.ID == 0 {
		return nil, detail, fmt.Errorf("Pixiv response for novel_id=%d does not contain novel.id", novelID)
	}
	return data, detail, nil
}

func (u *PixivUtils) GetBookmarkIllusts(user model.PixivUser, reqURL string) ([]byte, PixivBookmarkIllustResponse, error) {
	var payload PixivBookmarkIllustResponse
	data, err := u.GetJSONWithAuth(user, reqURL, &payload)
	return data, payload, err
}

func (u *PixivUtils) InitialBookmarkIllustURL(userID string, restrict string) string {
	values := url.Values{}
	values.Set("user_id", userID)
	values.Set("restrict", restrict)
	return "https://" + pixivAPIHost + "/v1/user/bookmarks/illust?" + values.Encode()
}

// PixivBookmarkNovelResponse mirrors the Pixiv /v1/user/bookmarks/novel response.
type PixivBookmarkNovelResponse struct {
	Novels  []PixivBookmarkNovel `json:"novels"`
	NextURL string               `json:"next_url"`
}

// PixivBookmarkNovel mirrors a single novel in the bookmark API response.
type PixivBookmarkNovel struct {
	ID         int64  `json:"id"`
	Title      string `json:"title"`
	Caption    string `json:"caption"`
	Restrict   int    `json:"restrict"`
	XRestrict  int    `json:"x_restrict"`
	IsOriginal bool   `json:"is_original"`
	ImageUrls  struct {
		SquareMedium string `json:"square_medium"`
		Medium       string `json:"medium"`
		Large        string `json:"large"`
	} `json:"image_urls"`
	CreateDate     string `json:"create_date"`
	TextLength     int    `json:"text_length"`
	TotalView      int    `json:"total_view"`
	TotalBookmarks int    `json:"total_bookmarks"`
	IsBookmarked   bool   `json:"is_bookmarked"`
	Visible        bool   `json:"visible"`
	IsMuted        bool   `json:"is_muted"`
	NovelAIType    int    `json:"novel_ai_type"`
	User           struct {
		ID               int64  `json:"id"`
		Name             string `json:"name"`
		Account          string `json:"account"`
		ProfileImageUrls struct {
			Medium string `json:"medium"`
		} `json:"profile_image_urls"`
		IsFollowed      bool `json:"is_followed"`
		IsAcceptRequest bool `json:"is_accept_request"`
	} `json:"user"`
	Tags []struct {
		Name           string  `json:"name"`
		TranslatedName *string `json:"translated_name"`
	} `json:"tags"`
	Series *struct {
		ID    int64  `json:"id"`
		Title string `json:"title"`
	} `json:"series"`
	PageCount     int `json:"page_count"`
	TotalComments int `json:"total_comments"`
}

func (u *PixivUtils) GetBookmarkNovels(user model.PixivUser, reqURL string) ([]byte, PixivBookmarkNovelResponse, error) {
	var payload PixivBookmarkNovelResponse
	data, err := u.GetJSONWithAuth(user, reqURL, &payload)
	return data, payload, err
}

func (u *PixivUtils) InitialBookmarkNovelURL(userID string, restrict string) string {
	values := url.Values{}
	values.Set("user_id", userID)
	values.Set("restrict", restrict)
	return "https://" + pixivAPIHost + "/v1/user/bookmarks/novel?" + values.Encode()
}

func (u *PixivUtils) GetJSONWithAuth(user model.PixivUser, reqURL string, target any) ([]byte, error) {
	data, status, err := u.doPixivAPIGet(reqURL, user.AccessToken)
	if err == nil && status == http.StatusOK {
		if err := json.Unmarshal(data, target); err != nil {
			return nil, fmt.Errorf("parse Pixiv response request=%s failed: %w", reqURL, err)
		}
		return data, nil
	}
	if !isAuthError(status, data) {
		if err != nil {
			return nil, err
		}
		return nil, fmt.Errorf("request=%s status=%d response=%s", reqURL, status, string(data))
	}

	freshUser := user
	if latest, ok := latestPixivUser(user.PixivUserID); ok && latest.AccessToken != "" {
		freshUser = latest
		if freshUser.AccessToken != user.AccessToken {
			data, status, err = u.doPixivAPIGet(reqURL, freshUser.AccessToken)
			if err != nil {
				return nil, err
			}
			if status == http.StatusOK {
				if err := json.Unmarshal(data, target); err != nil {
					return nil, fmt.Errorf("parse Pixiv response request=%s failed: %w", reqURL, err)
				}
				return data, nil
			}
			if !isAuthError(status, data) {
				return nil, fmt.Errorf("request=%s status=%d response=%s", reqURL, status, string(data))
			}
		}
	}

	newAccess, refreshErr := u.RefreshPixivToken(freshUser.PixivUserID, freshUser.RefreshToken)
	if refreshErr != nil {
		return nil, fmt.Errorf("token refresh failed: %w", refreshErr)
	}

	data, status, err = u.doPixivAPIGet(reqURL, newAccess)
	if err != nil {
		return nil, err
	}
	if status != http.StatusOK {
		return nil, fmt.Errorf("request=%s status=%d response=%s", reqURL, status, string(data))
	}
	if err := json.Unmarshal(data, target); err != nil {
		return nil, fmt.Errorf("parse Pixiv response request=%s failed: %w", reqURL, err)
	}
	return data, nil
}

func latestPixivUser(userID string) (model.PixivUser, bool) {
	var user model.PixivUser
	if err := db.DB.First(&user, "pixiv_user_id = ?", userID).Error; err != nil {
		return model.PixivUser{}, false
	}
	return user, true
}

func (u *PixivUtils) RefreshPixivToken(userID string, refreshToken string) (string, error) {
	timeStr := pixivClientTime()
	hash := pixivClientHash(timeStr)

	form := url.Values{}
	form.Set("client_id", "MOBrBDS8blbauoSck0ZfDbtuzpyT")
	form.Set("client_secret", "lsACyCD94FhDUtGTXi3QzcFE2uU1hqtDaKeqrdwj")
	form.Set("grant_type", "refresh_token")
	form.Set("refresh_token", refreshToken)
	form.Set("include_policy", "true")

	req, err := http.NewRequest("POST", "https://oauth.secure.pixiv.net/auth/token", strings.NewReader(form.Encode()))
	if err != nil {
		return "", err
	}

	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	setPixivAppHeaders(req, "")
	req.Header.Set("X-Client-Time", timeStr)
	req.Header.Set("X-Client-Hash", hash)

	resp, err := u.client().Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		bodyBytes, _ := io.ReadAll(resp.Body)
		return "", fmt.Errorf("request=%s status=%d response=%s", req.URL.String(), resp.StatusCode, string(bodyBytes))
	}

	var tokenResp struct {
		Response struct {
			AccessToken  string `json:"access_token"`
			RefreshToken string `json:"refresh_token"`
		} `json:"response"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&tokenResp); err != nil {
		return "", err
	}
	if tokenResp.Response.AccessToken == "" {
		return "", fmt.Errorf("received empty access token")
	}

	updates := map[string]any{
		"access_token": tokenResp.Response.AccessToken,
		"updated_at":   time.Now(),
	}
	if tokenResp.Response.RefreshToken != "" {
		updates["refresh_token"] = tokenResp.Response.RefreshToken
	}
	if err := db.DB.Model(&model.PixivUser{}).Where("pixiv_user_id = ?", userID).Updates(updates).Error; err != nil {
		slog.Error("Failed to update refreshed tokens", "userID", userID, "error", err)
	}
	return tokenResp.Response.AccessToken, nil
}

func (u *PixivUtils) DownloadFile(fileURL string, destPath string) error {
	req, err := http.NewRequest("GET", fileURL, nil)
	if err != nil {
		return err
	}
	req.Header.Set("Referer", "https://app-api.pixiv.net/")
	req.Header.Set("User-Agent", pixivAppUserAgent)

	resp, err := u.client().Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		bodyBytes, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("request=%s status=%d response=%s", fileURL, resp.StatusCode, string(bodyBytes))
	}

	out, err := os.Create(destPath)
	if err != nil {
		return err
	}
	defer out.Close()

	_, err = io.Copy(out, resp.Body)
	return err
}

func (u *PixivUtils) doPixivAPIGet(reqURL string, accessToken string) ([]byte, int, error) {
	req, err := http.NewRequest("GET", reqURL, nil)
	if err != nil {
		return nil, 0, err
	}
	setPixivAppHeaders(req, accessToken)
	req.Header.Set("Host", pixivAPIHost)

	resp, err := u.client().Do(req)
	if err != nil {
		return nil, 0, err
	}
	defer resp.Body.Close()

	data, readErr := io.ReadAll(resp.Body)
	if readErr != nil {
		return data, resp.StatusCode, readErr
	}
	return data, resp.StatusCode, nil
}

func (u *PixivUtils) client() *http.Client {
	if u != nil && u.Client != nil {
		return u.Client
	}
	return pixivClient
}

func setPixivAppHeaders(req *http.Request, accessToken string) {
	if accessToken != "" {
		req.Header.Set("Authorization", "Bearer "+accessToken)
	}
	timeStr := pixivClientTime()
	req.Header.Set("X-Client-Time", timeStr)
	req.Header.Set("X-Client-Hash", pixivClientHash(timeStr))
	req.Header.Set("User-Agent", pixivAppUserAgent)
	req.Header.Set("App-OS", pixivAppOS)
	req.Header.Set("App-OS-Version", pixivAppOSVersion)
	req.Header.Set("App-Version", pixivAppVersion)
}

func pixivClientTime() string {
	return time.Now().Format("2006-01-02T15:04:05") + "+00:00"
}

func pixivClientHash(t string) string {
	hashSalt := "28c1fdd170a5204386cb1313c7077b34f83e4aaf4aa829ce78c231e05b0bae2c"
	h := md5.Sum([]byte(t + hashSalt))
	return hex.EncodeToString(h[:])
}

func isAuthError(status int, body []byte) bool {
	if status == http.StatusUnauthorized {
		return true
	}
	if status == http.StatusBadRequest && bytes.Contains(body, []byte("invalid_grant")) {
		return true
	}
	return false
}
