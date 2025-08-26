# Simple helper to repeat a string n times
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

.activities[]
| (
    # "\(.user_id)\t" +
    "\(.created_at | to_localtime)\t" +
    "\(.country)\t" +
    "\(.provider)\t" +
    "\(.reference_id | pad_right_spaces(36))\t" +
    "\(.data.booking_type // "-" | pad_right_spaces(5))\t" +
    "\(.status)\t" +
    (
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
    + "\(.pricing.currency) " + "\(.pricing.total_price | pad_right_spaces(7))\t"
    + "\(.data.payment_method.type // "-" | pad_right_spaces(16))\t"
    + "\(.data.payment_profile // "-" | pad_right_spaces(8))"
  )

