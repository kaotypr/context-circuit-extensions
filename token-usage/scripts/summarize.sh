#!/bin/sh
# Totals a plan's token-usage entries per role. Reads the hook's JSON on stdin.
# Ends with one JSON line: status, summary, and details_path.

input=$(cat)

# field KEY prints the first JSON string value of "KEY", unescaped.
field() {
	printf '%s' "$input" | awk -v key="\"$1\":" '
	BEGIN { RS = "\001" }
	{
		i = index($0, key)
		if (i == 0) exit
		s = substr($0, i + length(key))
		if (substr(s, 1, 1) != "\"") exit
		out = ""
		for (j = 2; j <= length(s); j++) {
			c = substr(s, j, 1)
			if (c == "\"") break
			if (c == "\\") {
				j++
				c = substr(s, j, 1)
				if (c == "u") {
					hex = substr(s, j + 1, 4); j += 4
					if (hex == "0026") c = "&"
					else if (hex == "003c") c = "<"
					else if (hex == "003e") c = ">"
					else c = "?"
				} else if (c == "n") c = "\n"
				else if (c == "t") c = "\t"
			}
			out = out c
		}
		printf "%s", out
	}'
}

json_escape() {
	printf '%s' "$1" | awk 'BEGIN { RS = "\001" } {
		gsub(/\\/, "\\\\"); gsub(/"/, "\\\""); gsub(/\t/, "\\t"); gsub(/\n/, "\\n"); printf "%s", $0 }'
}

finish() { # status summary [details]
	if [ -n "$3" ]; then
		printf '{"status":"%s","summary":"%s","details_path":"%s"}\n' "$1" "$(json_escape "$2")" "$(json_escape "$3")"
	else
		printf '{"status":"%s","summary":"%s"}\n' "$1" "$(json_escape "$2")"
	fi
}

fail() {
	printf '%s\n' "$1" >&2
	exit 1
}

root=$(field workspace_root)
record=$(printf '%s' "$input" | awk 'BEGIN { RS = "\001" } {
	i = index($0, "\"record\":{\"id\":\"")
	if (i == 0) exit
	s = substr($0, i + 16); j = index(s, "\"")
	printf "%s", substr(s, 1, j - 1) }')
folder=$(field folder)
[ -n "$folder" ] || folder=.context-circuit/local/token-usage

[ -n "$root" ] || fail "no workspace_root in the hook input"
[ -n "$record" ] || { finish skipped "no record at this point, so there is no plan to total"; exit 0; }
case $record in
*[!a-z0-9-]*) fail "record id $record is not a plain ID" ;;
esac
case $folder in
/* | [A-Za-z]:* | \\*) fail "folder setting $folder is absolute; it must be relative to the workspace root" ;;
esac
case /$folder/ in
*/../* | */..\\* | *\\..\\* | *\\../*) fail "folder setting $folder leaves the workspace" ;;
esac

dir="$root/$folder"
log="$dir/$record.log"
if [ ! -f "$log" ]; then
	finish skipped "no entries recorded for $record"
	exit 0
fi
details="$dir/$record-summary.md"

summary=$(LC_ALL=C RECORD="$record" DETAILS="$details" awk '
BEGIN { record = ENVIRON["RECORD"]; details = ENVIRON["DETAILS"] }
function valid_tokens(t) { return t == "not-available" || t ~ /^[0-9]+$/ }
{
	sub(/\r$/, "")
	if ($0 ~ /^[ \t]*$/ || $0 ~ /^[ \t]*#/) next
	role = ""; tokens = ""
	for (i = 1; i <= NF; i++) {
		eq = index($i, "=")
		if (eq == 0) continue
		k = substr($i, 1, eq - 1); v = substr($i, eq + 1)
		if (k == "role") role = v
		else if (k == "tokens") tokens = v
	}
	if (role == "" || !valid_tokens(tokens)) { bad = bad (bad == "" ? "" : ", ") NR; nbad++; next }
	if (!(role in entries)) { roles[++nroles] = role }
	entries[role]++; total++
	if (tokens == "not-available") { na[role]++; nna++ } else { sum[role] += tokens; all += tokens }
}
END {
	# sort roles by name
	for (i = 2; i <= nroles; i++) { r = roles[i]; j = i - 1
		while (j > 0 && roles[j] > r) { roles[j + 1] = roles[j]; j-- }
		roles[j + 1] = r }
	print "# Token usage for " record > details
	print "" > details
	print "| Role | Entries | Tokens | Not available |" > details
	print "| --- | ---: | ---: | ---: |" > details
	line = ""
	for (i = 1; i <= nroles; i++) {
		r = roles[i]
		printf "| %s | %d | %d | %d |\n", r, entries[r], sum[r] + 0, na[r] + 0 > details
		line = line (i > 1 ? ", " : "") r " " (sum[r] + 0)
	}
	printf "| Total | %d | %d | %d |\n", total, all, nna > details
	print "" > details
	print "These totals cover only the " total " entries recorded. An entry the agent did not write is not counted, and a not-available entry adds no tokens." > details
	if (nbad > 0) { print "" > details; print "Lines not read: " bad "." > details }
	s = total " entries, " nna " not-available"
	if (line != "") s = s ": " line
	if (nbad > 0) s = s "; lines not read: " bad
	s = s "; totals cover only the entries recorded"
	print s
}' "$log") || fail "could not total $log"
finish ok "$summary" "$details"
