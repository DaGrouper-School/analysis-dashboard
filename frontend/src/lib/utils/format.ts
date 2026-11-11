/** Up to two initials for a photo placeholder: "Bay Street School" is "BS". */
export function initials(name: string): string {
	const words = name.split(/[\s,]+/).filter((w) => /^\p{Lu}/u.test(w));
	return (words.length ? words : name.split(/\s+/))
		.slice(0, 2)
		.map((w) => w[0]?.toUpperCase() ?? '')
		.join('');
}
