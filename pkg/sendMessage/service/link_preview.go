// Adapted from Evolution Go PR #207 (cateim); context, redirect and EXIF handling revised locally.
package send_service

import (
	"bytes"
	"context"
	"fmt"
	"image"
	"image/color"
	"image/jpeg"
	"net/http"
	neturl "net/url"
	"strings"
	"time"

	"go.mau.fi/whatsmeow"
	"go.mau.fi/whatsmeow/proto/waE2E"
	"golang.org/x/image/draw"
	"google.golang.org/protobuf/proto"
)

// linkPreviewUserAgent identifies preview requests as a link-preview crawler.
// Several storefronts only serve Open Graph tags to preview crawlers and answer
// Go's default client with a bot wall or an HTTP 403 (seen on Mercado Livre
// short links, Amazon and Shopee), which left the card without an image or
// titled after the error page.
const linkPreviewUserAgent = "WhatsApp/2.24.20.80 A"

const (
	// linkPreviewMaxSide caps the longest side of the high-quality thumbnail.
	linkPreviewMaxSide = 1200
	// linkPreviewInlineWidth is the width of the JPEGThumbnail embedded in the
	// message itself, shown while the high-quality thumbnail loads.
	linkPreviewInlineWidth   = 72
	linkPreviewMaxHTMLBytes  = 4 << 20
	linkPreviewMaxImageBytes = 8 << 20
	linkPreviewUploadTimeout = 30 * time.Second
	// linkPreviewMaxPixels bounds the decoded bitmap (4 bytes per pixel, about
	// 100 MB at the limit). The byte limit alone does not: a few kilobytes of
	// compressed image can declare a bitmap of several gigabytes.
	linkPreviewMaxPixels = 25_000_000
)

// linkPreviewData is the metadata rendered in the preview card.
type linkPreviewData struct {
	Title       string
	Description string
	ImageURL    string
}

// preparedLinkImage holds the preview image in the two sizes WhatsApp uses.
type preparedLinkImage struct {
	HighQuality []byte
	Inline      []byte
	Width       int
	Height      int
}

// linkPreviewTarget returns the URL the card points to: the explicit `url`
// field when given, otherwise the first URL found in the text.
func linkPreviewTarget(data *LinkStruct) string {
	if data.Url != "" {
		return data.Url
	}
	return findURL(data.Text)
}

// resolveLinkPreview starts from the metadata supplied by the caller and only
// fetches the page when the title or the image is missing. Caller values always
// win, so API users can build the card from data they already have and skip a
// page that refuses bots.
func resolveLinkPreview(data *LinkStruct, target string, fetch func(string) (string, string, string, error)) (linkPreviewData, error) {
	preview := linkPreviewData{Title: data.Title, Description: data.Description, ImageURL: data.ImgUrl}
	if target == "" || (preview.Title != "" && preview.ImageURL != "") {
		return preview, nil
	}

	title, description, imageURL, err := fetch(target)
	if err != nil {
		return preview, err
	}
	if preview.Title == "" {
		preview.Title = title
	}
	if preview.Description == "" {
		preview.Description = description
	}
	if preview.ImageURL == "" {
		preview.ImageURL = imageURL
	}
	return preview, nil
}

// absoluteURL resolves ref against base. It returns ref unchanged when either
// side cannot be parsed, and an empty string when ref is empty.
func absoluteURL(base, ref string) string {
	if ref == "" {
		return ""
	}
	baseURL, err := neturl.Parse(base)
	if err != nil {
		return ref
	}
	resolved, err := baseURL.Parse(ref)
	if err != nil {
		return ref
	}
	return resolved.String()
}

// prepareLinkPreviewImage decodes the preview image and encodes it as JPEG in
// the two sizes WhatsApp uses: the high-quality thumbnail (longest side capped
// at linkPreviewMaxSide) and the small inline JPEGThumbnail.
func prepareLinkPreviewImage(raw []byte) (preparedLinkImage, error) {
	if err := checkLinkPreviewDimensions(raw); err != nil {
		return preparedLinkImage{}, err
	}
	img, _, err := image.Decode(bytes.NewReader(raw))
	if err != nil {
		return preparedLinkImage{}, fmt.Errorf("link preview: the image could not be decoded: %w", err)
	}

	scaled := scaleToMaxSide(img, linkPreviewMaxSide)
	if orientation := jpegOrientation(raw); orientation > 1 {
		b := scaled.Bounds()
		rgba := image.NewRGBA(image.Rect(0, 0, b.Dx(), b.Dy()))
		draw.Draw(rgba, rgba.Bounds(), scaled, b.Min, draw.Src)
		scaled = orientRGBA(rgba, orientation)
	}
	// JPEG has no alpha: composite transparent pixels onto white.
	b := scaled.Bounds()
	opaque := image.NewRGBA(image.Rect(0, 0, b.Dx(), b.Dy()))
	draw.Draw(opaque, opaque.Bounds(), image.NewUniform(color.White), image.Point{}, draw.Src)
	draw.Draw(opaque, opaque.Bounds(), scaled, b.Min, draw.Over)
	scaled = opaque
	var highQuality bytes.Buffer
	if err := jpeg.Encode(&highQuality, scaled, &jpeg.Options{Quality: 85}); err != nil {
		return preparedLinkImage{}, fmt.Errorf("link preview: encoding the thumbnail: %w", err)
	}

	bounds := scaled.Bounds()
	return preparedLinkImage{
		HighQuality: highQuality.Bytes(),
		Inline:      makeJPEGThumbnail(highQuality.Bytes(), linkPreviewInlineWidth),
		Width:       bounds.Dx(),
		Height:      bounds.Dy(),
	}, nil
}

// checkLinkPreviewDimensions reads only the image header and rejects images
// whose decoded bitmap would exceed linkPreviewMaxPixels. WebP is covered too:
// the webp package imported by this service registers its decoder.
func checkLinkPreviewDimensions(raw []byte) error {
	cfg, _, err := image.DecodeConfig(bytes.NewReader(raw))
	if err != nil {
		return fmt.Errorf("link preview: the image could not be decoded: %w", err)
	}
	if cfg.Width <= 0 || cfg.Height <= 0 || uint64(cfg.Width) > uint64(linkPreviewMaxPixels)/uint64(cfg.Height) {
		return fmt.Errorf("link preview: a %dx%d image exceeds the %d pixel limit", cfg.Width, cfg.Height, linkPreviewMaxPixels)
	}
	return nil
}

// scaleToMaxSide shrinks img so its longest side is at most maxSide, keeping
// the aspect ratio. Smaller images are returned unchanged.
func scaleToMaxSide(img image.Image, maxSide int) image.Image {
	bounds := img.Bounds()
	width, height := bounds.Dx(), bounds.Dy()
	longest := max(width, height)
	if longest <= maxSide {
		return img
	}

	scaledWidth := max(1, width*maxSide/longest)
	scaledHeight := max(1, height*maxSide/longest)
	dst := image.NewRGBA(image.Rect(0, 0, scaledWidth, scaledHeight))
	draw.ApproxBiLinear.Scale(dst, dst.Bounds(), img, bounds, draw.Src, nil)
	return dst
}

// applyLinkThumbnailUpload fills the high-quality thumbnail fields from the
// upload result. Without them WhatsApp only has the inline JPEGThumbnail and
// renders the small, low-resolution card.
func applyLinkThumbnailUpload(ext *waE2E.ExtendedTextMessage, uploaded whatsmeow.UploadResponse, width, height int) {
	ext.ThumbnailDirectPath = proto.String(uploaded.DirectPath)
	ext.ThumbnailSHA256 = uploaded.FileSHA256
	ext.ThumbnailEncSHA256 = uploaded.FileEncSHA256
	ext.MediaKey = uploaded.MediaKey
	ext.MediaKeyTimestamp = proto.Int64(time.Now().Unix())
	ext.ThumbnailWidth = proto.Uint32(uint32(width))
	ext.ThumbnailHeight = proto.Uint32(uint32(height))
}

// prepareLinkMessage returns a usable text/card plus any optional preview error.
// The caller must abort on parent cancellation instead of sending the fallback.
func prepareLinkMessage(ctx context.Context, httpClient *http.Client, data *LinkStruct, upload func(context.Context, []byte) (whatsmeow.UploadResponse, error)) (*waE2E.Message, error) {
	ext := &waE2E.ExtendedTextMessage{Text: proto.String(data.Text)}
	msg := &waE2E.Message{ExtendedTextMessage: ext}
	target := linkPreviewTarget(data)
	if target == "" {
		return msg, nil
	}
	previewCtx, cancel := context.WithTimeout(ctx, 20*time.Second)
	defer cancel()
	preview, previewErr := resolveLinkPreview(data, target, func(url string) (string, string, string, error) {
		return fetchLinkMetadata(previewCtx, httpClient, url)
	})
	ext.MatchedText = proto.String(target)
	ext.Title = proto.String(preview.Title)
	ext.Description = proto.String(preview.Description)
	ext.PreviewType = waE2E.ExtendedTextMessage_NONE.Enum()
	if preview.ImageURL == "" {
		return msg, previewErr
	}
	raw, _, err := fetchLinkPreviewResource(previewCtx, httpClient, preview.ImageURL, linkPreviewMaxImageBytes)
	if err != nil {
		return msg, err
	}
	prepared, err := prepareLinkPreviewImage(raw)
	if err != nil {
		return msg, err
	}
	ext.JPEGThumbnail = prepared.Inline
	if strings.HasSuffix(data.Number, "@newsletter") {
		return msg, previewErr
	}
	uploaded, err := upload(previewCtx, prepared.HighQuality)
	if err != nil {
		return msg, err
	}
	applyLinkThumbnailUpload(ext, uploaded, prepared.Width, prepared.Height)
	return msg, previewErr
}
