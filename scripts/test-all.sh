#!/usr/bin/env bash
#
# test-all.sh — interactively walk picarctl through every command.
#
# For each command: describes what it's about to do, waits for you to press
# Enter, runs it, tells you what to expect, and asks whether it worked.
# Motor motion is capped at 1s (--duration 1s). Results are summarised at the end.
#
# Run from the gopicar directory:  ./scripts/test-all.sh

set -u

# Locate the binary relative to this script's parent (gopicar/).
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
GOPICAR_DIR="$(dirname "$SCRIPT_DIR")"
PICARCTL="$GOPICAR_DIR/bin/picarctl"

if [[ ! -x "$PICARCTL" ]]; then
    echo "ERROR: picarctl binary not found or not executable at: $PICARCTL"
    echo "Build it first (e.g. 'make' in $GOPICAR_DIR)."
    exit 1
fi

# Result tracking.
declare -a PASSED=()
declare -a FAILED=()
declare -a SKIPPED=()

# run_test <name> <description> <expected> <command...>
run_test() {
    local name="$1"; shift
    local desc="$1"; shift
    local expected="$1"; shift
    # remaining args are the command to run

    echo
    echo "=================================================================="
    echo "TEST: $name"
    echo "------------------------------------------------------------------"
    echo "About to: $desc"
    echo
    printf "Press Enter to run  (s = skip)... "
    read -r reply
    if [[ "$reply" == "s" || "$reply" == "S" ]]; then
        echo ">> skipped"
        SKIPPED+=("$name")
        return
    fi

    echo
    echo "\$ $PICARCTL $*"
    echo "------------------------------------------------------------------"
    "$PICARCTL" "$@"
    local rc=$?
    echo "------------------------------------------------------------------"
    echo "(exit code: $rc)"
    echo
    echo "EXPECTED: $expected"
    echo

    local ok
    while true; do
        printf "Did it work as expected? [y/n] "
        read -r ok
        case "$ok" in
            y|Y) PASSED+=("$name"); break ;;
            n|N)
                printf "  optional note about the failure: "
                local note
                read -r note
                if [[ -n "$note" ]]; then
                    FAILED+=("$name — $note")
                else
                    FAILED+=("$name")
                fi
                break ;;
            *) echo "  please answer y or n" ;;
        esac
    done
}

cat <<'EOF'
==================================================================
 picarctl — full command walkthrough
==================================================================
This will step through every picarctl command one at a time.
Keep hands clear of the wheels/servos. Motor runs are limited to 1s.
EOF

# --- Identity / bus -------------------------------------------------------
run_test "ping-mcu" \
    "probe the MCU over I2C" \
    "prints the MCU's I2C address and firmware version (no error)." \
    ping-mcu

run_test "hat-info" \
    "read HAT identity info" \
    "prints detected HAT version, speaker-EN pin, motor mode, and UUID." \
    hat-info

run_test "blink" \
    "blink the user LED (D14) 5 times" \
    "the onboard user LED blinks 5 times, then command exits cleanly." \
    blink --pin D14 --count 5

# --- Actuators: servos ----------------------------------------------------
run_test "servo pan" \
    "move the PAN (camera left/right) servo to 30 degrees" \
    "the pan servo rotates to +30 degrees." \
    servo --which pan --angle 30

run_test "servo tilt" \
    "move the TILT (camera up/down) servo to 20 degrees" \
    "the tilt servo tilts to +20 degrees." \
    servo --which tilt --angle 20

run_test "servo steer" \
    "steer the front wheels to 20 degrees" \
    "the front steering servo turns the wheels to +20 degrees." \
    servo --which steer --angle 20

run_test "servo center" \
    "return the steering servo to 0 (center)" \
    "the front wheels straighten back to center." \
    servo --which steer --angle 0

# --- Actuators: motors (capped at 1s) -------------------------------------
run_test "motor left" \
    "spin the LEFT rear motor forward at 50% for 1 second" \
    "the left rear wheel turns forward ~1s, then auto-stops." \
    motor --which left --speed 50 --duration 1s

run_test "motor right" \
    "spin the RIGHT rear motor forward at 50% for 1 second" \
    "the right rear wheel turns forward ~1s, then auto-stops." \
    motor --which right --speed 50 --duration 1s

run_test "stop" \
    "issue an explicit stop to both motors" \
    "both motors are stopped (already stopped; command exits cleanly)." \
    stop

# --- Sensors --------------------------------------------------------------
run_test "read-adc" \
    "read one raw ADC channel (A0)" \
    "prints a raw ADC value for channel A0." \
    read-adc --channel A0

run_test "read-battery" \
    "read the battery voltage" \
    "prints a plausible battery voltage (roughly 6-8.4V for a 2S pack)." \
    read-battery

run_test "read-grayscale" \
    "read the 3 grayscale line-follower channels" \
    "prints 3 grayscale values (A0/A1/A2)." \
    read-grayscale

run_test "distance" \
    "take one ultrasonic distance measurement" \
    "prints a distance in cm to whatever is in front of the sensor." \
    distance --count 1

# --- Reset (LAST: poisons live ADC state) ---------------------------------
run_test "reset-mcu" \
    "pulse the MCU reset line (run last: this poisons live ADC state)" \
    "the MCU resets without error. Sensor reads afterwards may be stale until re-init." \
    reset-mcu

# --- Summary --------------------------------------------------------------
echo
echo "=================================================================="
echo " SUMMARY"
echo "=================================================================="
echo "PASSED (${#PASSED[@]}):"
if [[ ${#PASSED[@]} -eq 0 ]]; then echo "  (none)"; else
    for t in "${PASSED[@]}"; do echo "  ✔ $t"; done
fi
echo
echo "FAILED (${#FAILED[@]}):"
if [[ ${#FAILED[@]} -eq 0 ]]; then echo "  (none)"; else
    for t in "${FAILED[@]}"; do echo "  x $t"; done
fi
echo
echo "SKIPPED (${#SKIPPED[@]}):"
if [[ ${#SKIPPED[@]} -eq 0 ]]; then echo "  (none)"; else
    for t in "${SKIPPED[@]}"; do echo "  - $t"; done
fi
echo "=================================================================="

# Exit non-zero if anything failed, so this is usable in a pipeline too.
[[ ${#FAILED[@]} -eq 0 ]]
