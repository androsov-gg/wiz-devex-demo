import React, { useState, useEffect } from 'react';
import { SidebarNav } from './components/SidebarNav';
import { TopHeader } from './components/TopHeader';
import { CatalogGrid } from './components/CatalogGrid';
import { InstanceTable } from './components/InstanceTable';
import { LaunchModal } from './components/LaunchModal';
import { LogsModal } from './components/LogsModal';
import { DemoTemplate, DemoInstance, UserSession } from './types';
import { Activity, CheckCircle2, Server, Sparkles } from 'lucide-react';

export default function App() {
  const [currentTab, setCurrentTab] = useState<'home' | 'catalog'>('home');
  const [searchQuery, setSearchQuery] = useState<string>('');

  const [templates, setTemplates] = useState<DemoTemplate[]>([]);
  const [instances, setInstances] = useState<DemoInstance[]>([]);
  const [isLoading, setIsLoading] = useState<boolean>(true);

  const [isLaunchModalOpen, setIsLaunchModalOpen] = useState<boolean>(false);
  const [selectedTemplate, setSelectedTemplate] = useState<DemoTemplate | null>(null);
  const [logsModalInstance, setLogsModalInstance] = useState<DemoInstance | null>(null);

  // Authenticated user representation
  const [user] = useState<UserSession>({
    email: 'ajkearns@altostrat.com',
    name: 'ajkearns',
    initials: 'AK',
    domain: 'altostrat.com',
  });

  // Fetch catalog templates
  useEffect(() => {
    fetch('/api/catalog')
      .then((res) => res.json())
      .then((data) => {
        if (data.catalog) setTemplates(data.catalog);
      })
      .catch((err) => console.error('Catalog fetch error:', err));
  }, []);

  // Fetch & poll active instances
  const fetchInstances = async () => {
    try {
      const res = await fetch('/api/instances');
      const data = await res.json();
      if (data.instances) {
        setInstances(data.instances);
      }
    } catch (err) {
      console.error('Instances fetch error:', err);
    } finally {
      setIsLoading(false);
    }
  };

  useEffect(() => {
    fetchInstances();
    const interval = setInterval(fetchInstances, 5000);
    return () => clearInterval(interval);
  }, []);

  // Search filtering (if header search query is entered)
  const displayInstances = searchQuery && currentTab === 'home'
    ? instances.filter((inst) => {
        const q = searchQuery.toLowerCase();
        return inst.templateName.toLowerCase().includes(q) || inst.id.toLowerCase().includes(q);
      })
    : instances;

  const displayTemplates = searchQuery && currentTab === 'catalog'
    ? templates.filter((tpl) => {
        const q = searchQuery.toLowerCase();
        return (
          tpl.name.toLowerCase().includes(q) ||
          tpl.description.toLowerCase().includes(q)
        );
      })
    : templates;

  const runningCount = instances.filter((i) => i.status === 'RUNNING').length;
  const provisioningCount = instances.filter(
    (i) => i.status === 'PROVISIONING' || i.status === 'BUILDING_IMAGE' || i.status === 'RESTARTING'
  ).length;

  const handleLaunchClick = (template: DemoTemplate) => {
    setSelectedTemplate(template);
    setIsLaunchModalOpen(true);
  };

  const handleRestart = async (instanceId: string) => {
    // Optimistically update the UI status immediately
    setInstances((prev) =>
      prev.map((inst) =>
        inst.id === instanceId
          ? { ...inst, status: 'RESTARTING' as const, cloudRunReadyAt: undefined }
          : inst
      )
    );
    try {
      await fetch(`/api/instances/${instanceId}/restart`, { method: 'POST' });
      fetchInstances();
    } catch (err) {
      console.error('Restart failed:', err);
      fetchInstances();
    }
  };

  const handleTeardown = async (instanceId: string) => {
    if (!confirm(`Are you sure you want to tear down instance ${instanceId}?`)) return;
    try {
      await fetch(`/api/instances/${instanceId}`, { method: 'DELETE' });
      setInstances((prev) => prev.filter((i) => i.id !== instanceId));
    } catch (err) {
      console.error(err);
    }
  };

  return (
    <div className="flex min-h-screen bg-slate-50">
      {/* Left Navigation Rail */}
      <SidebarNav currentTab={currentTab} onTabChange={setCurrentTab} />

      {/* Main Content Area */}
      <div className="flex-1 pl-20 flex flex-col min-w-0">
        {/* Top Header */}
        <TopHeader
          user={user}
          currentTab={currentTab}
          searchQuery={searchQuery}
          onSearchChange={setSearchQuery}
          onOpenLaunchModal={() => {
            setSelectedTemplate(null);
            setIsLaunchModalOpen(true);
          }}
        />

        {/* Page 1: Home (Active Demos Running & Launch Demo) */}
        {currentTab === 'home' && (
          <main className="p-8 space-y-6 flex-1">
            {/* Quick Metrics Bar */}
            <div className="grid grid-cols-1 sm:grid-cols-4 gap-4">
              <div className="bg-white p-4 rounded-xl border border-slate-200 shadow-2xs flex items-center gap-3">
                <div className="w-10 h-10 rounded-lg bg-blue-50 text-blue-600 flex items-center justify-center">
                  <Activity className="w-5 h-5" />
                </div>
                <div>
                  <div className="text-2xl font-bold text-slate-900">{instances.length}</div>
                  <div className="text-xs text-slate-500 font-medium">Active Environments</div>
                </div>
              </div>

              <div className="bg-white p-4 rounded-xl border border-slate-200 shadow-2xs flex items-center gap-3">
                <div className="w-10 h-10 rounded-lg bg-emerald-50 text-emerald-600 flex items-center justify-center">
                  <CheckCircle2 className="w-5 h-5" />
                </div>
                <div>
                  <div className="text-2xl font-bold text-slate-900">{runningCount}</div>
                  <div className="text-xs text-slate-500 font-medium">Ready & Running</div>
                </div>
              </div>

              <div className="bg-white p-4 rounded-xl border border-slate-200 shadow-2xs flex items-center gap-3">
                <div className="w-10 h-10 rounded-lg bg-amber-50 text-amber-600 flex items-center justify-center">
                  <Sparkles className="w-5 h-5" />
                </div>
                <div>
                  <div className="text-2xl font-bold text-slate-900">{provisioningCount}</div>
                  <div className="text-xs text-slate-500 font-medium">Provisioning Services</div>
                </div>
              </div>

              <div className="bg-white p-4 rounded-xl border border-slate-200 shadow-2xs flex items-center gap-3">
                <div className="w-10 h-10 rounded-lg bg-indigo-50 text-indigo-600 flex items-center justify-center">
                  <Server className="w-5 h-5" />
                </div>
                <div>
                  <div className="text-2xl font-bold text-slate-900">{templates.length}</div>
                  <div className="text-xs text-slate-500 font-medium">Catalog Templates</div>
                </div>
              </div>
            </div>

            {/* Active Instances Section */}
            <section className="space-y-3">
              <div className="flex items-center justify-between">
                <div>
                  <h2 className="text-base font-bold text-slate-900 tracking-tight">Active Environments</h2>
                  <p className="text-xs text-slate-500">
                    Real-time Cloud Run demo services secured with Google-Managed IAP
                  </p>
                </div>
                <div className="flex items-center gap-2 text-xs font-semibold text-emerald-600 bg-emerald-50 px-2.5 py-1 rounded-full border border-emerald-200">
                  <span className="w-2 h-2 rounded-full bg-emerald-500 animate-pulse" />
                  <span>Cloud Run Healthy</span>
                </div>
              </div>

              <InstanceTable
                instances={displayInstances}
                isLoading={isLoading}
                onOpenLogs={(inst) => setLogsModalInstance(inst)}
                onRestart={handleRestart}
                onTeardown={handleTeardown}
                onLaunchNew={() => {
                  setSelectedTemplate(null);
                  setIsLaunchModalOpen(true);
                }}
                onBrowseCatalog={() => setCurrentTab('catalog')}
              />
            </section>
          </main>
        )}

        {/* Page 2: Demo Catalog (Available Templates from GCS) */}
        {currentTab === 'catalog' && (
          <main className="p-8 space-y-6 flex-1">
            <div className="flex items-center justify-between">
              <div>
                <h2 className="text-base font-bold text-slate-900 tracking-tight">Demo Catalog</h2>
                <p className="text-xs text-slate-500">
                  Pre-configured demo templates ready for on-demand deployment
                </p>
              </div>
              <div className="text-xs text-slate-500 font-medium">
                {displayTemplates.length} {displayTemplates.length === 1 ? 'template' : 'templates'} available
              </div>
            </div>

            {/* Catalog Grid */}
            <CatalogGrid templates={displayTemplates} onLaunch={handleLaunchClick} />
          </main>
        )}
      </div>

      {/* Modals */}
      <LaunchModal
        isOpen={isLaunchModalOpen}
        templates={templates}
        selectedTemplate={selectedTemplate}
        onClose={() => setIsLaunchModalOpen(false)}
        onSuccess={(newInstance) => {
          setInstances((prev) => {
            const exists = prev.some((i) => i.id === newInstance.id);
            if (exists) {
              return prev.map((i) => (i.id === newInstance.id ? newInstance : i));
            }
            return [newInstance, ...prev];
          });
          setCurrentTab('home');
          fetchInstances();
        }}
      />

      <LogsModal
        instance={logsModalInstance}
        onClose={() => setLogsModalInstance(null)}
      />
    </div>
  );
}
