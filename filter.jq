# Filtering and presentation logic lies here

def repeat($s; $n):
  if $n > 0 then [range(0; $n)] | map($s) | add else "" end;

def pad_left_zeros($len):
  tostring
  | ($len - length) as $pad
  | if $pad > 0 then repeat("0"; $pad) + . else . end;

def pad_left_spaces($len):
  tostring
  | ($len - length) as $pad
  | if $pad > 0 then repeat(" "; $pad) + . else . end;

def pad_right_spaces($len):
  tostring
  | ($len - length) as $pad
  | if $pad > 0 then . + repeat(" "; $pad) else . end;

def to_localtime:
  fromdateiso8601
  | strflocaltime("%Y-%m-%d %H:%M:%S GMT%Z");

# ---------- ANSI coloring helpers ----------
def ESC: "\u001b[";                               # same as \x1b[
def C($n; $s): (ESC + ($n|tostring) + "m")        # wrap in color + reset
               + ($s|tostring)
               + (ESC + "0m");

# Maps: choose color code based on raw value, but color the padded display
def color_status($raw; $disp):
  ($raw|tostring|ascii_downcase) as $x
  | (if ($x | test("completed|success")) then 32          # green
     elif ($x | test("failed|error"))   then 31           # red
     elif ($x | test("ongoing|scheduled")) then 33          # yellow
     elif ($x | test("processing|in_progress")) then 33   # yellow
     else 90                                                  # dim
     end) as $c
  | C($c; $disp);

def color_provider($raw; $disp):
  ($raw|tostring|ascii_downcase) as $x
  | (if   $x == "careem-rides" then "92"  # bright green
     elif $x == "careem-rh"    then "94"  # bright blue
     elif $x == "hala-rides"   then "96"  # bright cyan
     elif $x == "careem-c4b-rides"   then "98"  # bright cyan
     else "90"                          # dim/default
     end) as $c
  | C($c; $disp);

def color_profile($raw; $disp):  # booking_type
  ($raw|tostring|ascii_downcase) as $x
  | (if ($x | test("business|corporate"))   then 36        # cyan
     elif ($x | test("personal|consumer"))  then 35        # magenta
     else 90
     end) as $c
  | C($c; $disp);

def color_booking_type($raw; $disp):
  ($raw|tostring|ascii_downcase) as $x
  | (if   ($x | test("now"))   then "1;32"   # bold green
     elif ($x | test("later")) then "34"     # blue
     else "90"                        # dim/default
     end) as $c
  | C($c; $disp);

def color_pm_type($raw; $disp):  # payment_method.type
  ($raw|tostring|ascii_downcase) as $x
  | (if    ($x | test("credit-card"))        then "32"   # green
     elif  ($x | test("cash"))               then "33"   # yellow
     elif  ($x | test("invoice"))            then "34"   # blue
     elif  ($x | test("delegated-wallet"))   then "96"   # bright cyan
     elif  ($x | test("digital-wallet"))     then "36"   # cyan
     else "90"
     end) as $c
  | C($c; $disp);

def color_payment_profile($raw; $disp):  # data.payment_profile
  ($raw|tostring|ascii_downcase) as $x
  | (if    ($x | test("company|business|corp")) then "94"  # bright blue
     elif  ($x | test("personal|consumer"))      then "96"  # bright cyan
     else "90"
     end) as $c
  | C($c; $disp);

def init_limit:
  (($ARGS.named.limit? // 10) | tonumber? // 10);

init_limit as $limit
| limit($limit; .activities[])
| (
    # capture raw values to classify
    .status as $status_raw
    | (.data.booking_type // "-") as $booking_type
    | (.data.payment_method.type // "-") as $type_raw
    | (.data.payment_profile // "-") as $pmprofile_raw
    | (.provider // "-") as $provider_raw
    | (.country // "-") as $country_raw

    # build line, padding first, then coloring selected fields
    | "\(.created_at | to_localtime)  "
    + "\($country_raw | pad_right_spaces(3))  "
    + (
        ($provider_raw | pad_right_spaces(16)) as $prov_disp | color_provider($provider_raw; $prov_disp)
      ) + "  "
    + "\(.reference_id | pad_right_spaces(36))  "
    + (
        ($booking_type | pad_right_spaces(5)) as $booking_type_disp | color_booking_type($booking_type; $booking_type_disp)
      ) + "  "
    + (
        ($status_raw | pad_right_spaces(9)) as $status_disp | color_status($status_raw; $status_disp)
      ) + "  "
    + (
        .data.distance // "__.__"
        | tostring
        | split(".")
        | (
            (.[0] | pad_left_spaces(3))
            + "."
            + ((.[1] // "00") | .[0:2])
          )
      )
    + " km\t"
    + "\(.pricing.currency) " + "\(.pricing.total_price | pad_right_spaces(7))  "
    + (
        ($type_raw | pad_right_spaces(16)) as $type_disp
        | color_pm_type($type_raw; $type_disp)
      ) + "  "
    + (
        ($pmprofile_raw | pad_right_spaces(8)) as $pmprof_disp
        | color_payment_profile($pmprofile_raw; $pmprof_disp)
      ) + "  "
    + "\(.data.cctid | pad_right_spaces(36))" + "  "
    + "\(.user_id)" + "  "
    + "\(.data.product.name // " " | gsub("\\s+"; "_"))"
  )

