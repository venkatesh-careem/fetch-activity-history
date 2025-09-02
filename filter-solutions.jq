
def repeat($s; $n):
  if $n > 0 then [range(0; $n)] | map($s) | add else "" end;

def pad_right_spaces($len):
  tostring
  | ($len - length) as $pad
  | if $pad > 0 then . + repeat(" "; $pad) else . end;

.data[] |

"\(.phoneNumber | pad_right_spaces(14))" +
"\(.userId | pad_right_spaces(10))" +
"\(.fullName | pad_right_spaces(24))" +
"\(.countryId // "-" | pad_right_spaces(4))" +
"\(.companyId // "-" | pad_right_spaces(6))" +
"\(.email | pad_right_spaces(54))" +
"\(.companyName // "-" | pad_right_spaces(32))" +
""
