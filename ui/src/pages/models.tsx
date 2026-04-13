import { useEffect, useState } from 'react';
import { useNavigate } from 'react-router-dom';
import { api } from '../api';
import { Box, Plus, Trash2, ArrowRight, Layers, Zap, X, ChevronRight, Link2, Search } from 'lucide-react';
import { ProviderLogoIcon } from '../components/connections/helpers';

function ProviderIcon({ provider, size = 16 }: { provider: string; size?: number }) {
    if (provider === 'combo') return <Layers size={size} className="text-[var(--purple)]" />;
    if (provider === 'alias') return <ArrowRight size={size} className="text-[var(--accent)]" />;
    return <ProviderLogoIcon provider={provider} size={size} />;
}

export default function Models() {
    const navigate = useNavigate();
    const [models, setModels] = useState<any[]>([]);
    const [aliases, setAliases] = useState<Record<string, string>>({});
    const [showAdd, setShowAdd] = useState(false);
    const [alias, setAlias] = useState('');
    const [model, setModel] = useState('');
    const [search, setSearch] = useState('');

    const load = () => {
        api.getModels()
            .then((data) => setModels(data || []))
            .catch(() => {});
        api.getAliases()
            .then((data) => setAliases(data || {}))
            .catch(() => {});
    };
    useEffect(() => {
        load();
    }, []);

    const handleAddAlias = async () => {
        if (!alias || !model) return;
        await api.setAlias(alias, model);
        setShowAdd(false);
        setAlias('');
        setModel('');
        load();
    };

    const handleDeleteAlias = async (name: string) => {
        await api.deleteAlias(name);
        load();
    };

    const kiroModels = models.filter((m: any) => m.provider === 'kiro');
    const openaiModels = models.filter((m: any) => m.provider === 'openai');
    const comboModels = models.filter((m: any) => m.provider === 'combo');

    const otherModels = models.filter((m: any) => !['kiro', 'openai', 'combo', 'alias'].includes(m.provider));

    // search filter
    const q = search.trim().toLowerCase();
    const filterModels = (list: any[]) => (!q ? list : list.filter((m: any) => m.id.toLowerCase().includes(q)));

    const sections = [
        {
            key: 'kiro',
            label: 'Kiro AI',
            models: filterModels(kiroModels),
            icon: <ProviderLogoIcon provider="kiro" size={18} />,
            colorClass: 'stat-card-green',
            accentColor: '#FF9900',
        },
        {
            key: 'openai',
            label: 'OpenAI',
            models: filterModels(openaiModels),
            icon: <ProviderLogoIcon provider="openai" size={18} />,
            colorClass: 'stat-card-blue',
            accentColor: '#10a37f',
        },
        {
            key: 'combo',
            label: 'Combos',
            models: filterModels(comboModels),
            icon: <Layers size={18} className="text-[var(--purple)]" />,
            colorClass: 'stat-card-purple',
            accentColor: '#a855f7',
        },
        {
            key: 'other',
            label: 'Other',
            models: filterModels(otherModels),
            icon: <Box size={18} className="text-[var(--text-muted)]" />,
            colorClass: '',
            accentColor: '#64748b',
        },
    ].filter((s) => s.models.length > 0);

    const filteredAliases = Object.entries(aliases).filter(
        ([a, m]) => !q || a.toLowerCase().includes(q) || m.toLowerCase().includes(q),
    );

    return (
        <div>
            {/* Header */}
            <div className="page-header">
                <div>
                    <h2 className="page-title">Models</h2>
                    <p className="page-subtitle">All available models across your providers, combos, and aliases.</p>
                </div>
                <div className="flex items-center gap-2 shrink-0">
                    <button onClick={() => navigate('/combos')} className="btn-ghost text-xs">
                        <Layers size={14} /> Manage Combos
                    </button>
                    <button onClick={() => navigate('/connections')} className="btn-ghost text-xs">
                        <Link2 size={14} /> Connections
                    </button>
                </div>
            </div>

            {/* Search */}
            <div className="glass-sm p-3 mb-6">
                <div className="relative">
                    <Search
                        size={15}
                        className="absolute left-3 top-1/2 -translate-y-1/2 text-[var(--text-dim)] pointer-events-none"
                    />
                    <input
                        type="search"
                        value={search}
                        onChange={(e) => setSearch(e.target.value)}
                        placeholder="Filter models by name or ID…"
                        className="glass-input w-full pl-9"
                    />
                </div>
            </div>

            {/* Summary stat */}
            <div className="grid grid-cols-2 sm:grid-cols-4 gap-3 mb-6">
                <div className="glass stat-card stat-card-green cursor-default">
                    <div className="text-[10px] uppercase font-bold tracking-wider text-[var(--text-dim)] mb-1 flex items-center gap-1.5">
                        <ProviderLogoIcon provider="kiro" size={12} /> Kiro
                    </div>
                    <div className="text-xl font-bold" style={{ fontFamily: 'var(--font-heading)' }}>
                        {kiroModels.length}
                    </div>
                </div>
                <div className="glass stat-card stat-card-blue cursor-default">
                    <div className="text-[10px] uppercase font-bold tracking-wider text-[var(--text-dim)] mb-1 flex items-center gap-1.5">
                        <ProviderLogoIcon provider="openai" size={12} /> OpenAI
                    </div>
                    <div className="text-xl font-bold" style={{ fontFamily: 'var(--font-heading)' }}>
                        {openaiModels.length}
                    </div>
                </div>
                <div className="glass stat-card stat-card-purple cursor-default">
                    <div className="text-[10px] uppercase font-bold tracking-wider text-[var(--text-dim)] mb-1 flex items-center gap-1.5">
                        <Layers size={12} className="text-[var(--purple)]" /> Combos
                    </div>
                    <div className="text-xl font-bold" style={{ fontFamily: 'var(--font-heading)' }}>
                        {comboModels.length}
                    </div>
                </div>
                <div className="glass stat-card stat-card-amber cursor-default">
                    <div className="text-[10px] uppercase font-bold tracking-wider text-[var(--text-dim)] mb-1 flex items-center gap-1.5">
                        <ArrowRight size={12} className="text-[var(--warning)]" /> Aliases
                    </div>
                    <div className="text-xl font-bold" style={{ fontFamily: 'var(--font-heading)' }}>
                        {Object.keys(aliases).length}
                    </div>
                </div>
            </div>

            {/* Provider sections */}
            {sections.map((section) => (
                <div key={section.key} className="mb-6">
                    <div className="flex items-center gap-2 mb-3">
                        <div
                            className="w-7 h-7 rounded-lg flex items-center justify-center"
                            style={{
                                backgroundColor: `${section.accentColor}15`,
                                border: `1px solid ${section.accentColor}25`,
                            }}
                        >
                            {section.icon}
                        </div>
                        <h3 className="text-sm font-semibold" style={{ fontFamily: 'var(--font-heading)' }}>
                            {section.label}
                        </h3>
                        <span className="chip chip-muted text-[10px]">{section.models.length}</span>
                        {section.key === 'combo' && (
                            <button
                                onClick={() => navigate('/combos')}
                                className="text-[10px] text-[var(--accent)] hover:text-[var(--accent-hover)] font-medium ml-auto flex items-center gap-0.5 cursor-pointer"
                            >
                                Manage <ChevronRight size={10} />
                            </button>
                        )}
                    </div>
                    <div className="flex flex-wrap gap-2">
                        {section.models.map((m: any) => (
                            <div
                                key={m.id}
                                className="glass-sm px-3 py-2 flex items-center gap-2 cursor-default hover:border-[var(--border-hover)] transition-all group"
                            >
                                <ProviderIcon provider={m.provider} size={14} />
                                <span className="font-mono text-xs text-[var(--text-muted)] group-hover:text-[var(--text)] transition-colors">
                                    {m.id}
                                </span>
                                {m.models?.length > 0 && (
                                    <span className="chip chip-purple text-[10px]">{m.models.length} models</span>
                                )}
                            </div>
                        ))}
                    </div>
                </div>
            ))}

            {/* Aliases */}
            <div className="mt-8">
                <div className="flex items-center justify-between mb-3">
                    <div className="flex items-center gap-2">
                        <div className="w-7 h-7 rounded-lg bg-[var(--accent-glow)] border border-[var(--accent)]/20 flex items-center justify-center">
                            <ArrowRight size={14} className="text-[var(--accent)]" />
                        </div>
                        <h3 className="text-sm font-semibold" style={{ fontFamily: 'var(--font-heading)' }}>
                            Aliases
                        </h3>
                        <span className="chip chip-muted text-[10px]">{Object.keys(aliases).length}</span>
                    </div>
                    <button onClick={() => setShowAdd(!showAdd)} className="btn-primary text-xs py-1.5 px-3">
                        <Plus size={14} /> Add Alias
                    </button>
                </div>

                {showAdd && (
                    <div className="glass p-4 mb-4 animate-slide-up">
                        <div className="flex gap-3 items-end flex-wrap">
                            <div className="flex-1 min-w-[120px]">
                                <label className="block text-xs text-[var(--text-muted)] mb-1.5 font-medium">
                                    Alias
                                </label>
                                <input
                                    value={alias}
                                    onChange={(e) => setAlias(e.target.value)}
                                    placeholder="sonnet"
                                    className="glass-input w-full"
                                />
                            </div>
                            <div className="flex-1 min-w-[180px]">
                                <label className="block text-xs text-[var(--text-muted)] mb-1.5 font-medium">
                                    Target Model
                                </label>
                                <input
                                    value={model}
                                    onChange={(e) => setModel(e.target.value)}
                                    placeholder="kiro/claude-sonnet-4.5"
                                    className="glass-input w-full"
                                />
                            </div>
                            <div className="flex items-center gap-2">
                                <button onClick={handleAddAlias} className="btn-primary py-[10px]">
                                    Save
                                </button>
                                <button
                                    onClick={() => {
                                        setShowAdd(false);
                                        setAlias('');
                                        setModel('');
                                    }}
                                    className="btn-icon"
                                >
                                    <X size={16} />
                                </button>
                            </div>
                        </div>
                    </div>
                )}

                {filteredAliases.length === 0 && !showAdd ? (
                    <div className="empty-state py-8">
                        <p className="text-sm text-[var(--text-muted)]">No aliases configured.</p>
                        <button
                            type="button"
                            onClick={() => setShowAdd(true)}
                            className="mt-3 text-xs text-[var(--accent)] hover:text-[var(--accent-hover)] font-medium cursor-pointer"
                        >
                            Create your first alias
                        </button>
                    </div>
                ) : (
                    <div className="space-y-2">
                        {filteredAliases.map(([a, m]) => (
                            <div
                                key={a}
                                className="glass-sm px-4 py-3 flex items-center justify-between hover:border-[var(--border-hover)] transition-all group"
                            >
                                <div className="flex items-center gap-3 min-w-0">
                                    <span className="font-mono text-sm font-medium text-[var(--text)]">{a}</span>
                                    <ArrowRight size={14} className="text-[var(--text-dim)] shrink-0" />
                                    <span className="font-mono text-sm text-[var(--text-muted)] truncate">{m}</span>
                                </div>
                                <button
                                    onClick={() => handleDeleteAlias(a)}
                                    className="btn-icon text-[var(--danger)] opacity-0 group-hover:opacity-100 transition-opacity hover:bg-[var(--danger-glow)]"
                                    title="Remove alias"
                                >
                                    <Trash2 size={14} />
                                </button>
                            </div>
                        ))}
                    </div>
                )}
            </div>

            {/* Cross-link tip */}
            <div className="glass-sm p-4 mt-6 flex flex-col sm:flex-row items-start sm:items-center justify-between gap-3 text-xs text-[var(--text-muted)]">
                <span>
                    Models are auto-discovered from your connections. Add more providers to expand your catalog.
                </span>
                <div className="flex items-center gap-3 shrink-0">
                    <button
                        onClick={() => navigate('/connections')}
                        className="text-[var(--accent)] hover:text-[var(--accent-hover)] font-medium flex items-center gap-1 cursor-pointer"
                    >
                        <Link2 size={12} /> Add Connection <ChevronRight size={12} />
                    </button>
                    <button
                        onClick={() => navigate('/playground')}
                        className="text-[var(--success)] hover:text-[var(--success)] font-medium flex items-center gap-1 cursor-pointer"
                    >
                        <Zap size={12} /> Try in Playground <ChevronRight size={12} />
                    </button>
                </div>
            </div>
        </div>
    );
}
