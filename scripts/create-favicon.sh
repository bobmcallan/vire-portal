#!/bin/bash
# Create favicon.ico for VIRE Portal
# Black background with white V letter, 16x16 32-bit BGRA

set -e

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_ROOT="$(dirname "$SCRIPT_DIR")"

STATIC_DIR="$PROJECT_ROOT/pages/static"
PAGES_DIR="$PROJECT_ROOT/pages"

mkdir -p "$STATIC_DIR"

create_favicon() {
    local black='\x00\x00\x00\xff'
    local white='\xff\xff\xff\xff'

    # ICO header
    printf '\x00\x00'       # Reserved
    printf '\x01\x00'       # Type (ICO)
    printf '\x01\x00'       # Image count

    # Directory entry
    printf '\x10'           # Width (16)
    printf '\x10'           # Height (16)
    printf '\x00'           # Colors (0 = true color)
    printf '\x00'           # Reserved
    printf '\x01\x00'       # Planes
    printf '\x20\x00'       # Bits per pixel (32)
    printf '\x68\x04\x00\x00'  # Image size (1128 bytes)
    printf '\x16\x00\x00\x00'  # Offset (22)

    # BMP header
    printf '\x28\x00\x00\x00'  # Header size (40)
    printf '\x10\x00\x00\x00'  # Width (16)
    printf '\x20\x00\x00\x00'  # Height (32, doubled for ICO)
    printf '\x01\x00'          # Planes
    printf '\x20\x00'          # Bits per pixel (32)
    printf '\x00\x00\x00\x00'  # Compression
    printf '\x00\x04\x00\x00'  # Image size
    printf '\x00\x00\x00\x00'  # X pixels/meter
    printf '\x00\x00\x00\x00'  # Y pixels/meter
    printf '\x00\x00\x00\x00'  # Colors used
    printf '\x00\x00\x00\x00'  # Important colors

    # Pixel data (16x16, bottom-up, BGRA)
    # V pattern: two diagonal strokes meeting at bottom center
    #
    #  Col: 0 1 2 3 4 5 6 7 8 9 A B C D E F
    #  R0:  . . . . . . . . . . . . . . . .   (top padding)
    #  R1:  . . . . . . . . . . . . . . . .
    #  R2:  . V V . . . . . . . . . V V . .
    #  R3:  . . V V . . . . . . . V V . . .
    #  R4:  . . V V . . . . . . . V V . . .
    #  R5:  . . . V V . . . . . V V . . . .
    #  R6:  . . . V V . . . . . V V . . . .
    #  R7:  . . . . V V . . . V V . . . . .
    #  R8:  . . . . V V . . . V V . . . . .
    #  R9:  . . . . . V V . V V . . . . . .
    # R10:  . . . . . V V . V V . . . . . .
    # R11:  . . . . . . V V V . . . . . . .
    # R12:  . . . . . . V V V . . . . . . .
    # R13:  . . . . . . . V . . . . . . . .
    # R14:  . . . . . . . . . . . . . . . .   (bottom padding)
    # R15:  . . . . . . . . . . . . . . . .

    # BMP is bottom-up, so row 15 first

    # Define rows as column ranges for white pixels
    # Row 15 (bottom padding)
    for col in $(seq 0 15); do printf "$black"; done
    # Row 14
    for col in $(seq 0 15); do printf "$black"; done
    # Row 13: col 7
    for col in $(seq 0 15); do
        if [ $col -eq 7 ]; then printf "$white"; else printf "$black"; fi
    done
    # Row 12: cols 6-8
    for col in $(seq 0 15); do
        if [ $col -ge 6 ] && [ $col -le 8 ]; then printf "$white"; else printf "$black"; fi
    done
    # Row 11: cols 6-8
    for col in $(seq 0 15); do
        if [ $col -ge 6 ] && [ $col -le 8 ]; then printf "$white"; else printf "$black"; fi
    done
    # Row 10: cols 5-6, 8-9
    for col in $(seq 0 15); do
        if ([ $col -ge 5 ] && [ $col -le 6 ]) || ([ $col -ge 8 ] && [ $col -le 9 ]); then printf "$white"; else printf "$black"; fi
    done
    # Row 9: cols 5-6, 8-9
    for col in $(seq 0 15); do
        if ([ $col -ge 5 ] && [ $col -le 6 ]) || ([ $col -ge 8 ] && [ $col -le 9 ]); then printf "$white"; else printf "$black"; fi
    done
    # Row 8: cols 4-5, 9-10
    for col in $(seq 0 15); do
        if ([ $col -ge 4 ] && [ $col -le 5 ]) || ([ $col -ge 9 ] && [ $col -le 10 ]); then printf "$white"; else printf "$black"; fi
    done
    # Row 7: cols 4-5, 9-10
    for col in $(seq 0 15); do
        if ([ $col -ge 4 ] && [ $col -le 5 ]) || ([ $col -ge 9 ] && [ $col -le 10 ]); then printf "$white"; else printf "$black"; fi
    done
    # Row 6: cols 3-4, 10-11
    for col in $(seq 0 15); do
        if ([ $col -ge 3 ] && [ $col -le 4 ]) || ([ $col -ge 10 ] && [ $col -le 11 ]); then printf "$white"; else printf "$black"; fi
    done
    # Row 5: cols 3-4, 10-11
    for col in $(seq 0 15); do
        if ([ $col -ge 3 ] && [ $col -le 4 ]) || ([ $col -ge 10 ] && [ $col -le 11 ]); then printf "$white"; else printf "$black"; fi
    done
    # Row 4: cols 2-3, 11-12
    for col in $(seq 0 15); do
        if ([ $col -ge 2 ] && [ $col -le 3 ]) || ([ $col -ge 11 ] && [ $col -le 12 ]); then printf "$white"; else printf "$black"; fi
    done
    # Row 3: cols 2-3, 11-12
    for col in $(seq 0 15); do
        if ([ $col -ge 2 ] && [ $col -le 3 ]) || ([ $col -ge 11 ] && [ $col -le 12 ]); then printf "$white"; else printf "$black"; fi
    done
    # Row 2: cols 1-2, 12-13
    for col in $(seq 0 15); do
        if ([ $col -ge 1 ] && [ $col -le 2 ]) || ([ $col -ge 12 ] && [ $col -le 13 ]); then printf "$white"; else printf "$black"; fi
    done
    # Row 1 (top padding)
    for col in $(seq 0 15); do printf "$black"; done
    # Row 0 (top padding)
    for col in $(seq 0 15); do printf "$black"; done

    # AND mask (64 bytes of zeros)
    for i in $(seq 1 64); do
        printf '\x00'
    done
}

echo "Creating favicon.ico..."
create_favicon > "$STATIC_DIR/favicon.ico"
cp "$STATIC_DIR/favicon.ico" "$PAGES_DIR/favicon.ico"

echo "Favicon created at:"
echo "  - $STATIC_DIR/favicon.ico"
echo "  - $PAGES_DIR/favicon.ico"
