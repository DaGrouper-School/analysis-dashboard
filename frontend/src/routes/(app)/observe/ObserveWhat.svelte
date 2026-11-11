<script lang="ts">
	import Icon from '#lib/components/Icon.svelte';
	import Select from '#lib/components/Select.svelte';
	import type {
		DeviceTypeDefinition,
		ObservationTarget,
		ObservationTemplate
	} from '#lib/domain/types.ts';
	import { FALLBACK_DOMAIN_ICON } from '#lib/utils/icons.ts';
	import { deviceTypeColor, deviceTypeIcon, deviceTypeLabel } from '#lib/utils/labels.ts';
	import { STEP_INFO, type TemplateGroup } from './observe-form.ts';

	/** The observation form's first step: the lab, and what is being recorded, by domain. */
	let {
		targets,
		target,
		labId = $bindable(),
		templateId,
		groups,
		deviceTypes,
		tried,
		onlab,
		onpick
	}: {
		targets: ObservationTarget[];
		target: ObservationTarget | null;
		labId: string;
		templateId: string;
		groups: TemplateGroup[];
		deviceTypes: DeviceTypeDefinition[];
		/** Next was tried: say so if nothing is picked. */
		tried: boolean;
		onlab: () => void;
		onpick: (templateId: string) => void;
	} = $props();

	/** How many of the lab's devices a template is read from, or null for a field observation. */
	const devicesOfType = (t: ObservationTemplate) =>
		t.deviceType ? (target?.devices.filter((d) => d.type === t.deviceType).length ?? 0) : null;
</script>

<div class="space-y-6">
	{#if targets.length > 1}
		<div class="max-w-sm">
			<label class="label" for="observe-lab">Lab</label>
			<Select
				id="observe-lab"
				options={targets.map((t) => ({
					value: t.lab.id,
					label: t.lab.name,
					hint: t.classroomName
				}))}
				bind:value={labId}
				onchange={onlab}
			/>
		</div>
	{:else if target}
		<p class="text-sm text-muted">
			Recording in <span class="text-ink">{target.lab.name}</span> · {target.classroomName}
		</p>
	{/if}
	<fieldset id="observe-what" tabindex="-1" class="outline-none">
		<legend class="flex items-center gap-2 text-lg font-semibold text-ink">
			<Icon name={STEP_INFO.what.icon} class="size-5 text-muted" /> What are you recording?
		</legend>
		<div class="mt-3 space-y-5">
			{#each groups as group (group.id)}
				<div>
					<p class="eyebrow flex items-center gap-1.5">
						<Icon name={group.icon} fallback={FALLBACK_DOMAIN_ICON} class="size-4" />
						{group.label}
					</p>
					<div class="mt-2 grid gap-2 sm:grid-cols-2">
						{#each group.templates as t (t.id)}
							{@const count = devicesOfType(t)}
							<label
								class={[
									'flex cursor-pointer gap-3 rounded-md border p-4 transition',
									t.id === templateId
										? 'border-ink bg-ink/[0.05]'
										: 'border-line hover:border-line-strong'
								]}
							>
								<input
									type="radio"
									name="template"
									class="mt-1 border-line-strong bg-transparent text-teal checked:bg-current focus:ring-teal"
									value={t.id}
									checked={t.id === templateId}
									onchange={() => onpick(t.id)}
								/>
								<span class="min-w-0">
									<span class="block font-medium text-ink">{t.label}</span>
									<span class="block text-sm text-muted">{t.description}</span>
									<span class="mt-1 block text-xs text-muted">
										{#if t.deviceType}
											<span
												class="mr-1 inline-flex align-[-2px]"
												style:color={deviceTypeColor(deviceTypes, t.deviceType)}
												><Icon
													name={deviceTypeIcon(deviceTypes, t.deviceType)}
													class="size-3.5"
												/></span
											>
											{deviceTypeLabel(deviceTypes, t.deviceType)} reading{count === 0
												? ' · none registered in this lab'
												: ''}
										{:else}
											Field observation, no device
										{/if}
									</span>
								</span>
							</label>
						{/each}
					</div>
				</div>
			{/each}
		</div>
	</fieldset>
	{#if tried && !templateId}
		<p class="field-error">Pick what you are recording.</p>
	{/if}
</div>
