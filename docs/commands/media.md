# media

`crabbox media` creates lightweight review artifacts from recorded desktop
videos. It runs locally and does not need a lease.

## Preview

`crabbox media preview` converts an MP4 or other ffmpeg-readable video into a
small animated GIF that GitHub can render inline in comments and pull request
bodies.

```sh
crabbox media preview \
  --input desktop.mp4 \
  --output desktop-preview.gif \
  --trimmed-video-output desktop-change.mp4
```

By default the preview is motion-focused:

- ffmpeg `freezedetect` finds leading and trailing static regions.
- Crabbox keeps a little padding around the first and last moving frame.
- The GIF is palette-optimized at 4 fps and 640 px wide.
- `--trimmed-video-output` writes an MP4 clip using the same motion window.

If no motion is detected, Crabbox keeps the full source video instead of
returning an empty preview.

Useful flags:

```text
--input <path>
--output <path>
--trimmed-video-output <path>
--width <px>              default 640
--fps <n>                 default 4
--trim-static             default true
--no-trim-static
--trim-padding <duration> default 750ms
--freeze-duration <dur>   default 500ms
--freeze-noise <level>    default -50dB
--min-duration <duration> default 1500ms
--json
```

`ffmpeg` and `ffprobe` must be on `PATH`.
