# StackChan app icon

The macOS application, shared settings header, login page and browser icon use
one original 3D/chibi interpretation of the square-screen StackChan desk robot.
The official product photo was inspected for the screen/head/base silhouette;
no official wordmark or product photograph is redistributed as our icon.

Source: `stackchan-server/server/internal/service/ai/assets/stackchan-icon.png`.
Web derivative: `stackchan-server/server/internal/service/ai/assets/stackchan-mark.png`
(256×256, embedded in the Go executable). The macOS builder derives all `.icns`
sizes from the source PNG. Resizing/format conversion preserves transparency.

Generation: built-in image generation; no CLI/API fallback. Final prompt:

> Use case: stylized-concept. Asset type: production macOS application icon and matching web GUI brand mascot, square 1024 by 1024. Primary request: an original charming 3D chibi interpretation of the StackChan desktop robot, with gentle anime-like personality. Subject: one compact ivory rounded-square screen head, glossy near-black display with two simple warm white oval eyes and a small happy smile, subtle rosy screen cheeks, a short visible mechanical pan-tilt neck sitting on a low dark rounded pedestal, no arms or legs. This must look like a small square-screen desk robot, not a humanoid or astronaut. Slight three-quarter pose but face clearly visible, head tilted inquisitively. Style: polished tactile 3D toy rendering, soft ceramic-like white casing, crisp uncomplicated silhouette, minimal hardware details, warm friendly expression. Palette: ivory and graphite robot with subtle periwinkle and teal light accents matching an existing dark navy/periwinkle settings GUI. Composition: a single centered icon, mascot large and instantly readable at small sizes, contained inside a dark navy softly rounded-square macOS-style tile; leave a modest even transparent outer margin around the tile. Background outside the tile genuinely transparent with alpha, not checkerboard. Soft studio lighting and restrained shadows. Constraints: one icon only, no lettering, no logos, no watermark, no extra characters, no accessories, no scenery, no UI mockup or icon grid. Preserve strong contrast of face and casing at 32px.

Product reference: https://docs.m5stack.com/en/StackChan
