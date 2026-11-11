import type { PublicDomainCard } from '#lib/domain/types.ts';

/**
 * The domain cards of one source (AP-45), such as the device picked on a classroom page: each
 * card keeps only that source's values, and a card left empty goes. Every source without one.
 */
export function domainsOf(cards: PublicDomainCard[], sourceId: string | null | undefined) {
	if (!sourceId) return cards;
	return cards
		.map((card) => ({ ...card, values: card.values.filter((v) => v.source.id === sourceId) }))
		.filter((card) => card.values.length);
}
