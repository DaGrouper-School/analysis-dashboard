/** Up to two initials for a photo placeholder: "Bay Street School" is "BS". */
export function initials(name: string): string {
	const words = name.split(/[\s,]+/).filter((w) => /^\p{Lu}/u.test(w));
	return (words.length ? words : name.split(/\s+/))
		.slice(0, 2)
		.map((w) => w[0]?.toUpperCase() ?? '')
		.join('');
}

/** The name to greet someone by, skipping a title: "Dr. Marcus L." is Marcus. */
export function givenName(fullName: string): string {
	return fullName.split(' ').find((part) => !/^(dr|prof|mr|mrs|ms)\.?$/i.test(part)) ?? fullName;
}
