package api

import (
	"net/http"

	"github.com/gin-gonic/gin"

	"github.com/photoprism/photoprism/internal/auth/acl"
	"github.com/photoprism/photoprism/internal/entity"
	"github.com/photoprism/photoprism/internal/entity/search"
	"github.com/photoprism/photoprism/internal/photoprism/cull"
	"github.com/photoprism/photoprism/internal/photoprism/get"
	"github.com/photoprism/photoprism/pkg/clean"
	"github.com/photoprism/photoprism/pkg/i18n"
	"github.com/photoprism/photoprism/pkg/rnd"
	"github.com/photoprism/photoprism/pkg/txt"
)

// CullResponse is a cull group with optional member photos.
type CullResponse struct {
	entity.Cull
	Members []CullMemberResponse `json:"Members,omitempty"`
}

// CullMemberResponse is a cull membership with photo search fields.
type CullMemberResponse struct {
	entity.PhotoCull
	Photo *search.Photo `json:"Photo,omitempty"`
}

// SearchCulls lists near-duplicate cull groups.
//
//	@Summary	lists near-duplicate cull groups
//	@Id			SearchCulls
//	@Tags		Culls
//	@Produce	json
//	@Success	200			{object}	entity.Culls
//	@Failure	401,403,429	{object}	i18n.Response
//	@Param		count		query		int		false	"maximum number of results"
//	@Param		offset		query		int		false	"result offset"
//	@Param		unreviewed	query		bool	false	"only groups without reviewed_at"
//	@Router		/api/v1/culls [get]
func SearchCulls(router *gin.RouterGroup) {
	router.GET("/culls", func(c *gin.Context) {
		s := Auth(c, acl.ResourcePhotos, acl.ActionSearch)

		if s.Abort(c) {
			return
		}

		count := txt.Int(c.Query("count"))
		if count <= 0 || count > 1000 {
			count = 100
		}

		offset := txt.Int(c.Query("offset"))
		if offset < 0 {
			offset = 0
		}

		q := entity.Db().Model(&entity.Cull{}).Order("created_at DESC")
		if c.Query("unreviewed") == "true" {
			q = q.Where("reviewed_at IS NULL")
		}

		var rows entity.Culls
		if err := q.Limit(count).Offset(offset).Find(&rows).Error; err != nil {
			AbortUnexpectedError(c)
			return
		}

		for i := range rows {
			if file, fileErr := entity.PrimaryFile(rows[i].KeeperUID); fileErr == nil && file != nil {
				rows[i].Thumb = file.FileHash
			}
		}

		AddCountHeader(c, len(rows))
		AddLimitHeader(c, count)
		AddOffsetHeader(c, offset)
		c.JSON(http.StatusOK, rows)
	})
}

// GetCull returns a cull group with members.
//
//	@Summary	returns a cull group with members
//	@Id			GetCull
//	@Tags		Culls
//	@Produce	json
//	@Success	200				{object}	api.CullResponse
//	@Failure	401,403,404,429	{object}	i18n.Response
//	@Param		uid				path		string	true	"cull UID"
//	@Router		/api/v1/culls/{uid} [get]
func GetCull(router *gin.RouterGroup) {
	router.GET("/culls/:uid", func(c *gin.Context) {
		s := Auth(c, acl.ResourcePhotos, acl.ActionView)

		if s.Abort(c) {
			return
		}

		uid := clean.UID(c.Param("uid"))
		if !rnd.IsUID(uid, entity.CullUID) {
			Abort(c, http.StatusNotFound, i18n.ErrNotFound)
			return
		}

		cullEntity := entity.FindCullByUID(uid)
		if cullEntity == nil {
			Abort(c, http.StatusNotFound, i18n.ErrNotFound)
			return
		}

		members, err := cullEntity.Members()
		if err != nil {
			AbortUnexpectedError(c)
			return
		}

		resp := CullResponse{Cull: *cullEntity, Members: make([]CullMemberResponse, 0, len(members))}

		for _, m := range members {
			item := CullMemberResponse{PhotoCull: m}

			var photo entity.Photo
			if err := entity.UnscopedDb().Where("photo_uid = ?", m.PhotoUID).First(&photo).Error; err == nil {
				result := search.Photo{}
				result.ID = photo.ID
				result.PhotoUID = photo.PhotoUID
				result.TakenAt = photo.TakenAt
				result.TakenAtLocal = photo.TakenAtLocal
				result.PhotoTitle = photo.PhotoTitle
				result.PhotoFavorite = photo.PhotoFavorite
				result.PhotoQuality = photo.PhotoQuality
				result.PhotoType = photo.PhotoType
				result.DeletedAt = photo.DeletedAt
				result.CullUID = cullEntity.CullUID
				result.CullRole = m.MemberRole

				if photo.DeletedAt != nil {
					result.DeletedAt = photo.DeletedAt
				}

				if file, fileErr := entity.PrimaryFile(photo.PhotoUID); fileErr == nil && file != nil {
					result.FileHash = file.FileHash
					result.FileWidth = file.FileWidth
					result.FileHeight = file.FileHeight
					result.FileUID = file.FileUID
				}

				item.Photo = &result
			}

			resp.Members = append(resp.Members, item)
		}

		c.JSON(http.StatusOK, resp)
	})
}

// SetCullKeeper promotes a member to keeper.
//
//	@Summary	sets the keeper photo for a cull group
//	@Id			SetCullKeeper
//	@Tags		Culls
//	@Accept		json
//	@Produce	json
//	@Success	200				{object}	entity.Cull
//	@Failure	400,401,403,404	{object}	i18n.Response
//	@Param		uid				path		string	true	"cull UID"
//	@Router		/api/v1/culls/{uid}/keeper [post]
func SetCullKeeper(router *gin.RouterGroup) {
	router.POST("/culls/:uid/keeper", func(c *gin.Context) {
		s := Auth(c, acl.ResourcePhotos, acl.ActionUpdate)

		if s.Abort(c) {
			return
		}

		uid := clean.UID(c.Param("uid"))
		cullEntity := entity.FindCullByUID(uid)
		if cullEntity == nil {
			Abort(c, http.StatusNotFound, i18n.ErrNotFound)
			return
		}

		var body struct {
			PhotoUID string `json:"PhotoUID"`
		}
		if err := c.BindJSON(&body); err != nil || !rnd.IsUID(body.PhotoUID, entity.PhotoUID) {
			AbortBadRequest(c, err)
			return
		}

		newKeeper := entity.FindPhoto(entity.Photo{PhotoUID: body.PhotoUID})
		if newKeeper == nil {
			Abort(c, http.StatusNotFound, i18n.ErrNotFound)
			return
		}
		_ = newKeeper.Restore()

		prevKeeperUID := cullEntity.KeeperUID
		if err := cullEntity.SetKeeper(body.PhotoUID); err != nil {
			AbortUnexpectedError(c)
			return
		}

		if get.Config().CullAutoArchive() && prevKeeperUID != "" && prevKeeperUID != body.PhotoUID {
			if prev := entity.FindPhoto(entity.Photo{PhotoUID: prevKeeperUID}); prev != nil && !prev.PhotoFavorite {
				if err := prev.Archive(); err == nil {
					_ = entity.Db().Model(&entity.PhotoCull{}).
						Where("cull_uid = ? AND photo_uid = ?", uid, prevKeeperUID).
						Updates(entity.Values{"member_role": entity.CullRoleReject, "auto_archived": true})
				}
			}
		}

		c.JSON(http.StatusOK, cullEntity)
	})
}

// ProtectCullMember marks a member as manually kept.
//
//	@Summary	protects a cull member from auto-archive
//	@Id			ProtectCullMember
//	@Tags		Culls
//	@Accept		json
//	@Produce	json
//	@Success	200				{object}	entity.Cull
//	@Failure	400,401,403,404	{object}	i18n.Response
//	@Param		uid				path		string	true	"cull UID"
//	@Router		/api/v1/culls/{uid}/keep [post]
func ProtectCullMember(router *gin.RouterGroup) {
	router.POST("/culls/:uid/keep", func(c *gin.Context) {
		s := Auth(c, acl.ResourcePhotos, acl.ActionUpdate)

		if s.Abort(c) {
			return
		}

		uid := clean.UID(c.Param("uid"))
		cullEntity := entity.FindCullByUID(uid)
		if cullEntity == nil {
			Abort(c, http.StatusNotFound, i18n.ErrNotFound)
			return
		}

		var body struct {
			PhotoUID string `json:"PhotoUID"`
		}
		if err := c.BindJSON(&body); err != nil || !rnd.IsUID(body.PhotoUID, entity.PhotoUID) {
			AbortBadRequest(c, err)
			return
		}

		if err := cullEntity.ProtectMember(body.PhotoUID); err != nil {
			AbortUnexpectedError(c)
			return
		}

		c.JSON(http.StatusOK, cullEntity)
	})
}

// RestoreCull restores auto-archived rejects in a cull group.
//
//	@Summary	restores auto-archived rejects in a cull group
//	@Id			RestoreCull
//	@Tags		Culls
//	@Produce	json
//	@Success	200			{object}	i18n.Response
//	@Failure	401,403,404	{object}	i18n.Response
//	@Param		uid			path		string	true	"cull UID"
//	@Router		/api/v1/culls/{uid}/restore [post]
func RestoreCull(router *gin.RouterGroup) {
	router.POST("/culls/:uid/restore", func(c *gin.Context) {
		s := Auth(c, acl.ResourcePhotos, acl.ActionUpdate)

		if s.Abort(c) {
			return
		}

		uid := clean.UID(c.Param("uid"))
		if err := cull.RestoreGroup(uid); err != nil {
			Abort(c, http.StatusNotFound, i18n.ErrNotFound)
			return
		}

		c.JSON(http.StatusOK, i18n.Response{Code: http.StatusOK})
	})
}

// DeleteCull dissolves a cull group without deleting files.
//
//	@Summary	dissolves a cull group
//	@Id			DeleteCull
//	@Tags		Culls
//	@Produce	json
//	@Success	200			{object}	i18n.Response
//	@Failure	401,403,404	{object}	i18n.Response
//	@Param		uid			path		string	true	"cull UID"
//	@Router		/api/v1/culls/{uid} [delete]
func DeleteCull(router *gin.RouterGroup) {
	router.DELETE("/culls/:uid", func(c *gin.Context) {
		s := Auth(c, acl.ResourcePhotos, acl.ActionDelete)

		if s.Abort(c) {
			return
		}

		uid := clean.UID(c.Param("uid"))
		if err := cull.Dissolve(uid); err != nil {
			Abort(c, http.StatusNotFound, i18n.ErrNotFound)
			return
		}

		c.JSON(http.StatusOK, i18n.Response{Code: http.StatusOK})
	})
}
