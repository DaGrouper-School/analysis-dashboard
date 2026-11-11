/**
 * A picked image file, scaled down so its longer side is at most `maxSide` pixels, as a JPEG data
 * URL. Keeps uploads small whatever camera took the photo.
 */
export async function scaledImage(file: File, maxSide = 1200, quality = 0.85): Promise<string> {
	const bitmap = await createImageBitmap(file);
	const scale = Math.min(1, maxSide / Math.max(bitmap.width, bitmap.height));
	const canvas = document.createElement('canvas');
	canvas.width = Math.round(bitmap.width * scale);
	canvas.height = Math.round(bitmap.height * scale);
	canvas.getContext('2d')!.drawImage(bitmap, 0, 0, canvas.width, canvas.height);
	bitmap.close();
	return canvas.toDataURL('image/jpeg', quality);
}
