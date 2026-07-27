package commands

import (
	"fmt"
	"time"

	"github.com/dustin/go-humanize/english"
	"github.com/urfave/cli/v2"

	"github.com/photoprism/photoprism/internal/config"
	"github.com/photoprism/photoprism/internal/entity"
	"github.com/photoprism/photoprism/internal/workers"
	"github.com/photoprism/photoprism/pkg/txt/report"
)

// CullCommands configures the near-duplicate cull subcommands.
var CullCommands = &cli.Command{
	Name:  "cull",
	Usage: "Near-duplicate / burst cull subcommands",
	Subcommands: []*cli.Command{
		CullRunCommand,
		CullLsCommand,
	},
}

// CullRunCommand runs near-duplicate detection.
var CullRunCommand = &cli.Command{
	Name:  "run",
	Usage: "Detects near-duplicate bursts and optionally soft-archives non-keepers",
	Flags: []cli.Flag{
		&cli.BoolFlag{
			Name:    "force",
			Aliases: []string{"f"},
			Usage:   "reprocess photos already in unreviewed auto cull groups",
		},
		&cli.IntFlag{
			Name:    "count",
			Aliases: []string{"n"},
			Usage:   "maximum number of candidate photos to consider",
			Value:   50000,
		},
	},
	Action: cullRunAction,
}

// CullLsCommand lists existing cull groups.
var CullLsCommand = &cli.Command{
	Name:  "ls",
	Usage: "Lists near-duplicate cull groups",
	Flags: append(report.CliFlags, &cli.IntFlag{
		Name:    "count",
		Aliases: []string{"n"},
		Usage:   "maximum number of groups to list",
		Value:   100,
	}),
	Action: cullLsAction,
}

// cullRunAction starts the cull worker.
func cullRunAction(ctx *cli.Context) error {
	return CallWithDependencies(ctx, func(conf *config.Config) error {
		worker := workers.NewCull(conf)
		return worker.Start(ctx.Int("count"), ctx.Bool("force"))
	})
}

// cullLsAction prints cull groups as a table.
func cullLsAction(ctx *cli.Context) error {
	return CallWithDependencies(ctx, func(conf *config.Config) error {
		limit := ctx.Int("count")
		if limit <= 0 {
			limit = 100
		}

		var rows entity.Culls
		if err := entity.Db().Order("created_at DESC").Limit(limit).Find(&rows).Error; err != nil {
			return err
		}

		out := make([][]string, 0, len(rows))
		for _, c := range rows {
			reviewed := ""
			if c.ReviewedAt != nil {
				reviewed = c.ReviewedAt.Format(time.RFC3339)
			}
			out = append(out, []string{
				c.CullUID,
				c.KeeperUID,
				fmt.Sprintf("%d", c.MemberCount),
				fmt.Sprintf("%.2f", c.CullScore),
				c.CullSrc,
				reviewed,
				c.CreatedAt.Format(time.RFC3339),
			})
		}

		fmt.Printf("%s\n", english.Plural(len(out), "cull group", "cull groups"))
		result, err := report.RenderFormat(out, []string{"UID", "Keeper", "Members", "Score", "Src", "Reviewed", "Created"}, report.CliFormat(ctx))
		if err != nil {
			return err
		}
		fmt.Printf("%s\n", result)
		return nil
	})
}
