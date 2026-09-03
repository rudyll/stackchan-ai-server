# StackChan crown-and-wings artwork

The selected identity is the hardware-faithful, box-shaped StackChan with its
original dot eyes and straight mouth, a simple gold hand-drawn crown and
cream/blue hand-drawn angel wings. Later elaborate-crown and smiling/sparkling-eye
candidates are not used. The robot retains its polished 3D materials and low
notched base; the accessories are intentionally 2D doodles.

## Shared files

| Use | File | Size |
| --- | --- | --- |
| Master / macOS ICNS input | `stackchan-server/server/internal/service/ai/assets/stackchan-icon.png` | 1254×1254 |
| Embedded GUI header, login, favicon | `stackchan-server/server/internal/service/ai/assets/stackchan-mark.png` | 256×256 |
| HA store icon | `stackchan-server/icon.png` | 128×128 |
| HA store logo and README artwork | `stackchan-server/logo.png` | 256×256 |

All PNGs have transparent outer corners; the navy tile and dark screen remain
visible rather than being cut out. The macOS builder produces the full ICNS size set from the master.
The GUI keeps its existing 64px header and 88px login images and relative URLs,
including under HA Ingress; no external image host or new public API is needed.

The [HA presentation guide](https://developers.home-assistant.io/docs/apps/presentation/)
requires PNG files named `icon.png` and `logo.png`, a square icon, and recommends
a 128px icon. It permits other logo aspect ratios, so the same square artwork is
used instead of a separate wordmark. These files control the store presentation,
not HA's navigation sidebar icon. No new HA add-on version is published with the
macOS-only artwork release; already installed server binaries keep their bundled
GUI image until updated/rebuilt.

After replacing the master on macOS, regenerate committed derivatives:

```bash
bash scripts/sync-brand-assets.sh
```

The AI-package asset tests check PNG sizes, real corner transparency (at most
1/255 alpha), a near-opaque interior (at least 250/255 alpha) and identical
GUI/HA logo bytes. These small tolerances accommodate extraction/resampling.
README/package tests guard image paths
and the shared source. Keep local concept files out of Git.

## Design provenance and prompts

Created and refined with the built-in imagegen tool, without CLI/API fallback.
The [official product reference](https://docs.m5stack.com/en/StackChan) informed
the physical silhouette. No official photograph or wordmark is redistributed
as the icon.

### Selected crown-and-wings edit

Use case: precise-object-edit.
Asset type: updated StackChan macOS app icon concept, square.
Input image 1: EDIT TARGET, the user's chosen B design. Preserve this exact robot, its hardware geometry, face, proportions, front three-quarter camera angle, ivory cuboid shell, black screen, two tiny white dot eyes, straight short white mouth, subtle ports, low dark gray NOTCHED two-foot base, 3D Pixar-like material finish and warm/cool lighting. Preserve the navy rounded-square icon tile. Do not redesign the machine.

Primary request: add a HAND-DRAWN CROWN and a pair of HAND-DRAWN ANGEL WINGS to this 3D robot. Mixed media: robot remains polished 3D, accessories are obviously charming 2D doodles.
Crown: one small three-point crown floating just above the top of the machine, playfully tilted a little. Warm golden-yellow hand-drawn marker strokes with slight natural irregularity and a subtle pale-gold flat fill, simple and readable, no jewels, no metallic 3D rendering. Crown width about 25% of robot head width, comfortably inside icon tile.
Wings: a matched pair of small white/ivory illustrated angel wings behind the upper-middle sides of the robot, extending outward and slightly upward, left and right. Bold gently wobbly cream hand-drawn outer strokes, 3 or 4 simple rounded feather lobes each, sparse pale-blue pencil-like interior feather lines and minimal flat fill. These should look drawn onto the image, NOT realistic feathers or 3D attachments. Wings emerge from BEHIND body edges and never cover the screen or side ports. They should be clearly visible on dark navy. No arms.
Composition: preserve original viewing direction, materials, face and enclosure. Uniformly scale robot down only as necessary to give crown and both wings breathing space. Keep all wing tips, crown and entire notched base inside the icon tile, balanced margins. Robot remains dominant.
Background cleanup: remove the baked-in gray-white checkerboard outside the rounded tile; outside the tile should be genuine transparent alpha, not a painted transparency grid. Inside the tile keep the same dark navy with soft blue glow.
Constraints: ONLY add the crown and wings plus necessary fit/background cleanup. Do not add halo, text, logos, sparkles, hearts, blush, big eyes, ears, humanoid limbs or a round pedestal. Do not turn the robot into a drawing. No cropping. No checkerboard pattern anywhere.

### Production transparency cleanup

Use case: background-extraction.
Input image 1: EDIT TARGET, approved final StackChan crown-and-angel-wings icon.
This is a PRODUCTION EXPORT cleanup, NOT a redesign. Preserve EVERY visible pixel of the navy rounded-square tile and everything inside it: robot exact original dot eyes/straight mouth, ivory cuboid shell, charcoal split-foot base, golden simple hand-drawn crown, cream/blue hand-drawn angel wings, lighting, composition and size.
ONLY remove the black outside-corner background surrounding the rounded-square icon tile. Make that exterior genuinely TRANSPARENT with an alpha channel. Do not draw a checkerboard or solid white/black backdrop; deliver an actual transparent PNG cutout of the entire rounded-square tile. Preserve antialiased rounded edges. Do NOT remove the navy background INSIDE the tile, dark screen or base. No cropping, relighting, retouching, repainting or adding features. Square canvas, original layout. The approved crown and wings and neutral face must remain unchanged.
