import React, { useState, useEffect } from 'react';
import { useApp } from '../../context/AppContext';
import { Trash2, ClipboardList } from 'lucide-react';

export default function DeleteIntentsModal() {
  const { makeAuthenticatedRequest, showToast } = useApp();
  const [isOpen, setIsOpen] = useState(false);
  const [intentIds, setIntentIds] = useState('');

  useEffect(() => {
    const handleOpen = () => {
      setIsOpen(true);
    };
    
    window.addEventListener('openDeleteIntentsModal', handleOpen);
    return () => window.removeEventListener('openDeleteIntentsModal', handleOpen);
  }, []);

  useEffect(() => {
    const handleKeyDown = (e) => {
      if (e.key === 'Escape' && isOpen) {
        handleClose();
      }
    };
    document.addEventListener('keydown', handleKeyDown);
    return () => document.removeEventListener('keydown', handleKeyDown);
  }, [isOpen]);

  const handleClose = () => {
    setIsOpen(false);
    setIntentIds('');
  };

  const handleOverlayClick = (e) => {
    if (e.target === e.currentTarget) {
      handleClose();
    }
  };

  const handleDelete = async () => {
    if (!intentIds.trim()) {
      showToast('error', 'Please enter intent IDs');
      return;
    }
    
    const ids = intentIds.split(',').map(id => id.trim()).filter(id => id);
    
    if (ids.length === 0) {
      showToast('error', 'No valid intent IDs provided');
      return;
    }
    
    try {
      const response = await makeAuthenticatedRequest('/api/v1/intents', {
        method: 'DELETE',
        body: JSON.stringify({ intentIds: ids })
      });
      
      const data = await response.json();
      
      if (data.success) {
        showToast('success', `Deleted ${ids.length} intent(s)`);
        handleClose();
        window.dispatchEvent(new CustomEvent('refreshIntents'));
      } else {
        showToast('error', data.error || data.message || 'Failed to delete intents');
      }
    } catch (error) {
      showToast('error', 'Error: ' + error.message);
    }
  };

  if (!isOpen) return null;

  return (
    <div className={`modal-overlay ${isOpen ? 'active' : ''}`} onClick={handleOverlayClick}>
      <div className="modal-container">
        <div className="modal-header">
          <h2>
            <span className="modal-icon"><Trash2 size={18} /></span>
            Delete Schedule Intents
          </h2>
          <button className="modal-close" onClick={handleClose}>×</button>
        </div>
        <div className="modal-body">
          <div className="input-group">
            <label htmlFor="intentIds">
              <span className="label-icon"><ClipboardList size={14} /></span>
              Intent IDs (comma separated)
            </label>
            <textarea 
              id="intentIds" 
              rows="3" 
              placeholder="intent-id-1, intent-id-2"
              value={intentIds}
              onChange={(e) => setIntentIds(e.target.value)}
            ></textarea>
            <div className="input-glow"></div>
          </div>
          <div className="form-actions">
            <button type="button" className="danger-btn" onClick={handleDelete}>
              <span className="btn-text">Delete Intents</span>
            </button>
          </div>
        </div>
      </div>
    </div>
  );
}
