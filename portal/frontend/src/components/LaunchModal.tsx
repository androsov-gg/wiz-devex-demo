import React, { useState, useEffect } from 'react';
import { X, Play, Loader2, Sparkles, AlertCircle } from 'lucide-react';
import { DemoTemplate, DemoInstance } from '../types';

interface LaunchModalProps {
  isOpen: boolean;
  templates: DemoTemplate[];
  selectedTemplate?: DemoTemplate | null;
  onClose: () => void;
  onSuccess: (instance: DemoInstance) => void;
}

export function LaunchModal({
  isOpen,
  templates,
  selectedTemplate,
  onClose,
  onSuccess,
}: LaunchModalProps) {
  const [templateId, setTemplateId] = useState<string>('');
  const [selectedVersion, setSelectedVersion] = useState<string>('1.0.0');
  const [customName, setCustomName] = useState<string>('');
  const [isLaunching, setIsLaunching] = useState<boolean>(false);
  const [launchStep, setLaunchStep] = useState<string>('');
  const [buildId, setBuildId] = useState<string | null>(null);
  const [error, setError] = useState<string | null>(null);

  useEffect(() => {
    if (selectedTemplate) {
      setTemplateId(selectedTemplate.id);
      setSelectedVersion(selectedTemplate.selectedVersion || selectedTemplate.versions?.[0]?.version || '1.0.0');
    } else if (templates.length > 0 && !templateId) {
      setTemplateId(templates[0].id);
      setSelectedVersion(templates[0].selectedVersion || templates[0].versions?.[0]?.version || '1.0.0');
    }
  }, [selectedTemplate, templates]);

  const activeTemplate = templates.find((t) => t.id === templateId) || templates[0];

  useEffect(() => {
    if (activeTemplate?.versions && activeTemplate.versions.length > 0) {
      const currentVerExists = activeTemplate.versions.some((v) => v.version === selectedVersion);
      if (!currentVerExists) {
        setSelectedVersion(activeTemplate.versions[0].version);
      }
    }
  }, [templateId, activeTemplate]);

  // Poll Cloud Build if image compilation was triggered
  useEffect(() => {
    let interval: ReturnType<typeof setInterval>;
    if (buildId && isLaunching) {
      interval = setInterval(async () => {
        try {
          const res = await fetch(`/api/builds?buildId=${buildId}`);
          const data = await res.json();
          if (data.status === 'SUCCESS') {
            setLaunchStep('Build complete! Deploying Cloud Run service...');
            clearInterval(interval);
          } else if (data.status === 'FAILURE') {
            setError('Cloud Build failed. Check build logs.');
            setIsLaunching(false);
            clearInterval(interval);
          }
        } catch (e) {
          console.error(e);
        }
      }, 4000);
    }
    return () => {
      if (interval) clearInterval(interval);
    };
  }, [buildId, isLaunching]);

  if (!isOpen) return null;

  const currentVersionObj = activeTemplate?.versions?.find((v) => v.version === selectedVersion);
  const displayDescription = currentVersionObj?.description || activeTemplate?.description;

  const handleLaunch = async (e: React.FormEvent) => {
    e.preventDefault();
    setIsLaunching(true);
    setError(null);
    setLaunchStep('Checking Artifact Registry for pre-built image...');

    try {
      const res = await fetch('/api/instances', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({
          templateId,
          version: selectedVersion,
          customName: customName.trim() || undefined,
        }),
      });

      const data = await res.json();

      if (!res.ok) {
        throw new Error(data.error || 'Failed to launch demo');
      }

      if (data.status === 'BUILDING_IMAGE' && data.buildId) {
        setBuildId(data.buildId);
        setLaunchStep(`Image (${templateId}:${selectedVersion}) not found in registry. Triggering Cloud Build from GCS configs (~60-90s)...`);
        setTimeout(() => {
          onSuccess(data.instance);
          onClose();
        }, 3000);
      } else {
        setLaunchStep('Image found! Deploying Cloud Run service...');
        setTimeout(() => {
          onSuccess(data.instance);
          onClose();
        }, 1200);
      }
    } catch (err: any) {
      setError(err.message);
      setIsLaunching(false);
    }
  };

  return (
    <div className="fixed inset-0 z-50 bg-slate-900/40 backdrop-blur-xs flex items-center justify-center p-4">
      <div className="bg-white rounded-2xl border border-slate-200 shadow-2xl max-w-lg w-full overflow-hidden animate-in fade-in zoom-in-95 duration-150">
        {/* Modal Header */}
        <div className="flex items-center justify-between px-6 py-4 border-b border-slate-100">
          <div className="flex items-center gap-2">
            <div className="w-8 h-8 rounded-lg bg-blue-50 text-blue-600 flex items-center justify-center">
              <Sparkles className="w-4 h-4" />
            </div>
            <div>
              <h2 className="text-sm font-bold text-slate-900">Launch Demo Environment</h2>
              <p className="text-[11px] text-slate-500">Deploy an isolated Cloud Run demo service with IAP security</p>
            </div>
          </div>
          <button
            onClick={onClose}
            disabled={isLaunching}
            className="p-1 rounded-lg text-slate-400 hover:text-slate-700 hover:bg-slate-100 transition-colors"
          >
            <X className="w-4 h-4" />
          </button>
        </div>

        {/* Modal Form */}
        <form onSubmit={handleLaunch} className="p-6 space-y-4">
          {error && (
            <div className="p-3 bg-rose-50 border border-rose-200 rounded-lg flex items-start gap-2 text-xs text-rose-700">
              <AlertCircle className="w-4 h-4 text-rose-600 shrink-0 mt-0.5" />
              <span>{error}</span>
            </div>
          )}

          {/* Template & Version Selectors */}
          <div className="grid grid-cols-1 sm:grid-cols-3 gap-2.5">
            <div className="sm:col-span-2">
              <label className="block text-xs font-semibold text-slate-700 mb-1.5">
                Select Demo Template
              </label>
              <select
                value={templateId}
                onChange={(e) => setTemplateId(e.target.value)}
                disabled={isLaunching}
                className="w-full px-3 py-2 text-xs bg-slate-50 border border-slate-200 rounded-lg focus:outline-none focus:ring-2 focus:ring-blue-500/20 focus:border-blue-500"
              >
                {templates.map((tpl) => (
                  <option key={tpl.id} value={tpl.id}>
                    {tpl.name}
                  </option>
                ))}
              </select>
            </div>

            <div>
              <label className="block text-xs font-semibold text-slate-700 mb-1.5">
                Version
              </label>
              <select
                value={selectedVersion}
                onChange={(e) => setSelectedVersion(e.target.value)}
                disabled={isLaunching || !activeTemplate?.versions?.length}
                className="w-full px-3 py-2 text-xs bg-slate-50 border border-slate-200 rounded-lg focus:outline-none focus:ring-2 focus:ring-blue-500/20 focus:border-blue-500"
              >
                {activeTemplate?.versions && activeTemplate.versions.length > 0 ? (
                  activeTemplate.versions.map((v) => (
                    <option key={v.version} value={v.version}>
                      {v.version}
                    </option>
                  ))
                ) : (
                  <option value="v1">v1</option>
                )}
              </select>
            </div>
          </div>

          {/* Selected Template Details Preview */}
          {activeTemplate && (
            <div className="p-3.5 bg-slate-50 rounded-xl border border-slate-200/80 text-xs space-y-2">
              <p className="text-slate-600 leading-relaxed text-[11px]">
                {displayDescription}
              </p>
              <div className="flex items-center gap-3 text-[10px] text-slate-500 font-medium pt-1">
                <span>⚡ {activeTemplate.cpu} vCPU</span>
                <span>🧠 {activeTemplate.memory} RAM</span>
                <span>📦 Storage Bucket Config</span>
              </div>
            </div>
          )}

          {/* Custom Instance Identifier (Optional) */}
          <div>
            <label className="block text-xs font-semibold text-slate-700 mb-1.5">
              Custom Instance Name (Optional)
            </label>
            <input
              type="text"
              placeholder="e.g. acme-corp-demo"
              value={customName}
              onChange={(e) => setCustomName(e.target.value)}
              disabled={isLaunching}
              className="w-full px-3 py-2 text-xs bg-slate-50 border border-slate-200 rounded-lg focus:outline-none focus:ring-2 focus:ring-blue-500/20 focus:border-blue-500 placeholder:text-slate-400"
            />
          </div>

          {/* Launch Progress Feedback */}
          {isLaunching && (
            <div className="p-3 bg-blue-50 border border-blue-200 rounded-xl space-y-2">
              <div className="flex items-center gap-2 text-xs font-semibold text-blue-700">
                <Loader2 className="w-3.5 h-3.5 animate-spin" />
                <span>{launchStep}</span>
              </div>
              <p className="text-[11px] text-blue-600/80">
                Cloud Run is provisioning your isolated demo environment. You can close this modal; the instance will appear in your active table.
              </p>
            </div>
          )}

          {/* Actions */}
          <div className="flex items-center justify-end gap-2 pt-2">
            <button
              type="button"
              onClick={onClose}
              disabled={isLaunching}
              className="px-4 py-2 text-xs font-semibold text-slate-600 hover:bg-slate-100 rounded-lg transition-colors"
            >
              Cancel
            </button>
            <button
              type="submit"
              disabled={isLaunching}
              className="px-5 py-2 text-xs font-semibold bg-blue-600 hover:bg-blue-700 text-white rounded-lg shadow-sm flex items-center gap-2 transition-all disabled:opacity-50"
            >
              {isLaunching ? (
                <>
                  <Loader2 className="w-3.5 h-3.5 animate-spin" />
                  <span>Launching...</span>
                </>
              ) : (
                <>
                  <Play className="w-3.5 h-3.5 fill-current" />
                  <span>Launch Environment</span>
                </>
              )}
            </button>
          </div>
        </form>
      </div>
    </div>
  );
}
