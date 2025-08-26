# Simple helper to repeat a string n times
def repeat($s; $n):
  if $n > 0 then [range(0; $n)] | map($s) | add else "" end;

def pad_left_zeros($len):
  tostring
  | ($len - length) as $pad
  | if $pad > 0 then repeat("0"; $pad) + . else . end;

def pad_right_spaces($len):
  tostring
  | ($len - length) as $pad
  | if $pad > 0 then . + repeat(" "; $pad) else . end;

.activities[]
| (
    "\(.provider)\t" +
    "\(.reference_id | pad_right_spaces(36))\t" +
    "\(.user_id)\t" +
    "\(.status)\t" +
    "\(.country)\t" +
    "\(.data.booking_type // "-" | pad_right_spaces(5))\t" +
    (
      .data.distance // "__.__"
      | tostring
      | split(".")
      | (
          (.[0] | pad_left_zeros(2))
          + "."
          + ((.[1] // "00") | .[0:2])
        )
    )
    + " km\t"
    + "\(.data.payment_method.type // "_")"
  )

