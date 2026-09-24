package main

import (
	"encoding/json"
	"fmt"
	"io"
	"text/tabwriter"
	"time"

	pkgApi "cbox/pkg/api"
)

func PrintJSON(w io.Writer, data any) error {
	b, err := json.MarshalIndent(data, "", "  ")
	if err != nil {
		return err
	}
	_, err = fmt.Fprintln(w, string(b))
	return err
}

func PrintContainerTable(w io.Writer, containers []pkgApi.ContainerResponse) {
	tw := tabwriter.NewWriter(w, 0, 0, 3, ' ', 0)
	fmt.Fprintln(tw, "CONTAINER ID\tNAME\tIMAGE\tGPU\tSTATUS\tCREATED")

	for _, c := range containers {
		idShort := c.ID
		if len(idShort) > 12 {
			idShort = idShort[:12]
		}

		createdStr := timeSince(c.CreatedAt)

		status := c.State
		if c.State == "running" && c.StartedAt != nil {
			status = fmt.Sprintf("Up %s", timeSince(*c.StartedAt))
		} else if c.ExitCode != nil {
			status = fmt.Sprintf("Exited (%d)", *c.ExitCode)
		}

		gpu := c.GPU
		if gpu == "" {
			gpu = "default"
		}

		fmt.Fprintf(tw, "%s\t%s\t%s\t%s\t%s\t%s\n",
			idShort, c.Name, c.ImageName, gpu, status, createdStr,
		)
	}
	tw.Flush()
}

func PrintImageTable(w io.Writer, images []pkgApi.ImageResponse) {
	tw := tabwriter.NewWriter(w, 0, 0, 3, ' ', 0)
	fmt.Fprintln(tw, "IMAGE ID\tTAGS\tBASE\tCREATED")

	for _, img := range images {
		tags := "<none>"
		if len(img.Tags) > 0 {
			tags = fmt.Sprintf("%v", img.Tags)
		}
		fmt.Fprintf(tw, "%s\t%s\t%s\t%s\n",
			img.ID, tags, img.Base, timeSince(img.CreatedAt),
		)
	}
	tw.Flush()
}

func PrintVolumeTable(w io.Writer, volumes []pkgApi.VolumeResponse) {
	tw := tabwriter.NewWriter(w, 0, 0, 3, ' ', 0)
	fmt.Fprintln(tw, "VOLUME NAME\tMODE\tSOURCE\tCREATED")

	for _, v := range volumes {
		fmt.Fprintf(tw, "%s\t%s\t%s\t%s\n",
			v.Name, v.Mode, v.Source, timeSince(v.CreatedAt),
		)
	}
	tw.Flush()
}

func PrintContextTable(w io.Writer, contexts []pkgApi.ContextResponse) {
	tw := tabwriter.NewWriter(w, 0, 0, 3, ' ', 0)
	fmt.Fprintln(tw, "CURRENT\tNAME\tPROVIDER\tPROFILE\tAUTO SCHEDULE")

	for _, c := range contexts {
		currentMarker := ""
		if c.IsCurrent {
			currentMarker = "*"
		}
		fmt.Fprintf(tw, "%s\t%s\t%s\t%s\t%v\n",
			currentMarker, c.Name, c.Provider, c.Profile, c.AutoSchedule,
		)
	}
	tw.Flush()
}

func timeSince(t time.Time) string {
	d := time.Since(t)
	if d < time.Minute {
		return fmt.Sprintf("%d seconds ago", int(d.Seconds()))
	}
	if d < time.Hour {
		return fmt.Sprintf("%d minutes ago", int(d.Minutes()))
	}
	if d < 24*time.Hour {
		return fmt.Sprintf("%d hours ago", int(d.Hours()))
	}
	return fmt.Sprintf("%d days ago", int(d.Hours()/24))
}
