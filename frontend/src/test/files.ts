/** A small PNG, as a file someone would pick. */
export async function pngFile(name = 'school.png'): Promise<File> {
	const canvas = document.createElement('canvas');
	canvas.width = 32;
	canvas.height = 18;
	const context = canvas.getContext('2d')!;
	context.fillStyle = '#2a7';
	context.fillRect(0, 0, 32, 18);
	const blob = await new Promise<Blob>((done) => canvas.toBlob((b) => done(b!), 'image/png'));
	return new File([blob], name, { type: 'image/png' });
}
